/*
 * Copyright 2017-2018 IBM Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lcm

import (
	"github.com/IBM/FfDL/commons/config"
	"github.com/cenkalti/backoff"
	"strconv"
	//"github.com/IBM/FfDL/commons/metricsmon"
	"github.com/IBM/FfDL/lcm/service/lcm/helper"
	"github.com/IBM/FfDL/lcm/service/lcm/learner"
	arbv1 "github.com/kubernetes-incubator/kube-arbitrator/pkg/apis/v1alpha1"
	"k8s.io/api/apps/v1beta1"
	v1core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"time"
)

func (t xqueuejobTraining) Start() error {

	serviceSpec := learner.CreateServiceSpec(t.learner.name, t.req.TrainingId)

	numLearners := int(t.req.GetResources().Learners)
	qjSpec := t.QJPodSpecForLearner(serviceSpec.Name)

	return t.CreateFromBOM(&xqueuejobTrainingBOM{
		t.learner.secrets,
		serviceSpec,
		t.helper.sharedVolumeClaim,
		qjSpec,
		t.QJSpecForHelper(),
		numLearners,
	})
}

///-------
func (t xqueuejobTraining) QJSpecForHelper() *arbv1.XQueueJob {

	helperDefn := t.helper
	helperContainers := t.constructAuxillaryContainers()

	podSpec := helper.CreateQJHelperPodSpec(helperContainers, []v1core.Volume{helperDefn.etcdVolume, helperDefn.sharedVolume}, map[string]string{"training_id": t.req.TrainingId, "user_id": t.req.UserId}, helperDefn.name)
	qjSpec := helper.CreateQJForHelper(helperDefn.name, podSpec, t.req.TrainingId, t.req.UserId)
	return qjSpec

}

///-------
func (t xqueuejobTraining) deploymentSpecForHelper() *v1beta1.Deployment {

	helperDefn := t.helper
	helperContainers := t.constructAuxillaryContainers()

	podSpec := helper.CreatePodSpec(helperContainers, []v1core.Volume{helperDefn.etcdVolume, helperDefn.sharedVolume}, map[string]string{"training_id": t.req.TrainingId, "user_id": t.req.UserId})
	deploymentSpec := helper.CreateDeploymentForHelper(helperDefn.name, podSpec)
	return deploymentSpec

}

func (t xqueuejobTraining) QJPodSpecForLearner(serviceName string) *arbv1.XQueueJob {

	logr := t.logr
	gpus := make(map[string]string)
	if t.req.Resources.Gpus > 0 {
		gpus["ibm-cloud.kubernetes.io/gpu-type"] = t.req.Resources.GpuType
	}

	learnerDefn := t.learner
	helperDefn := t.helper

	helperAndLearnerVolumes := append(learnerDefn.volumes, helperDefn.sharedVolume)

	//now create the learner container
	learnerContainer := constructLearnerContainer(t.req, learnerDefn.envVars, learnerDefn.volumeMounts, helperDefn.sharedVolumeMount, learnerDefn.mountTrainingDataStoreInLearner, learnerDefn.mountResultsStoreInLearner, t.logr) // nil for mounting shared NFS volume since non split mode
	logr.Infof("Added app: %s", t.req.TrainingId)
	numLearnersStr := strconv.Itoa(int(t.req.GetResources().Learners))
	logr.Infof("Added size: %s", numLearnersStr)
	learnerPodSpec := learner.CreateQJPodSpec([]v1core.Container{learnerContainer}, helperAndLearnerVolumes, map[string]string{"training_id": t.req.TrainingId, "user_id": t.req.UserId, "app": t.req.TrainingId, "size": numLearnersStr}, gpus, t.learner.name)
	learnerQJSpec := learner.CreateQJSpecForLearner(learnerDefn.name, serviceName, learnerDefn.numberOfLearners, learnerPodSpec, t.req.TrainingId, t.req.UserId)

	return learnerQJSpec
}

// this also creates the learner pod spec
func (t xqueuejobTraining) statefulSetSpecForLearner(serviceName string) (*v1beta1.StatefulSet, error) {

	gpus := make(map[string]string)
	if t.req.Resources.Gpus > 0 {
		gpus["ibm-cloud.kubernetes.io/gpu-type"] = t.req.Resources.GpuType
	}

	learnerDefn := t.learner
	helperDefn := t.helper

	helperAndLearnerVolumes := append(learnerDefn.volumes, helperDefn.sharedVolume)

	imagePullSecret, err := learner.GenerateImagePullSecret(t.k8sClient, t.req)
	if err != nil {
		return nil, err
	}

	//now create the learner container
	learnerContainer := constructLearnerContainer(t.req, learnerDefn.envVars, learnerDefn.volumeMounts, helperDefn.sharedVolumeMount, learnerDefn.mountTrainingDataStoreInLearner, learnerDefn.mountResultsStoreInLearner, t.logr) // nil for mounting shared NFS volume since non split mode
	splitLearnerPodSpec := learner.CreatePodSpec([]v1core.Container{learnerContainer}, helperAndLearnerVolumes, map[string]string{"training_id": t.req.TrainingId, "user_id": t.req.UserId}, gpus, imagePullSecret)
	statefulSetSpec := learner.CreateStatefulSetSpecForLearner(learnerDefn.name, serviceName, learnerDefn.numberOfLearners, splitLearnerPodSpec)

	return statefulSetSpec, nil
}

//CreateFromBOM ... eventually use with controller and make this transactional
func (t *xqueuejobTraining) CreateFromBOM(bom *xqueuejobTrainingBOM) error {
	logr := t.logr

	namespace := config.GetLearnerNamespace()

	//create shared volume
	if bom.sharedVolumeClaimBOM != nil { //if nil then must be static volume claim and does not need to be dynamically bound
		logr.Infof("Split training with shared volume claim %s not nil, creating shared PVC for training", bom.sharedVolumeClaimBOM.Name)
		if err := backoff.RetryNotify(func() error {
			return helper.CreatePVCFromBOM(bom.sharedVolumeClaimBOM, t.k8sClient)
		}, k8sInteractionBackoff(), func(err error, window time.Duration) {
			logr.WithError(err).Errorf("Failed in creating shared volume claim %s while deploying for training ", bom.sharedVolumeClaimBOM.Name)
			k8sFailureCounter.With(component, "volume").Add(1)
		}); err != nil {
			return err
		}
	}

	//create helper
	if err := backoff.RetryNotify(func() error {
		_, err := t.karClient.ArbV1().XQueueJobs(config.GetPodNamespace()).Create(bom.helperBOM)
		if k8serrors.IsAlreadyExists(err) {
			logr.WithError(err).Warnf("QJfor helper %s already exists", bom.helperBOM.Name)
			return nil
		}
		return err
	}, k8sInteractionBackoff(), func(err error, window time.Duration) {
		logr.WithError(err).Errorf("Failed in creating helper %s while deploying for training", bom.helperBOM.Name)
		k8sFailureCounter.With(component, "helper").Add(1)
	}); err != nil {
		return err
	}

	for _, secret := range bom.secrets {
		//create the secrets
		if err := backoff.RetryNotify(func() error {
			_, err := t.k8sClient.CoreV1().Secrets(namespace).Create(secret)
			if k8serrors.IsAlreadyExists(err) {
				logr.WithError(err).Warnf("secret %s already exists", secret.Name)
				return nil
			}
			return err
		}, k8sInteractionBackoff(), func(err error, window time.Duration) {
			logr.WithError(err).Errorf("Failed in creating secret %s while deploying for training ", secret.Name)
			k8sFailureCounter.With(component, "secret").Add(1)
		}); err != nil {
			return err
		}
		logr.Infof("Created secret %s", secret.Name)
	}

	if bom.numLearners > 1 {
		//create service
		if err := backoff.RetryNotify(func() error {
			_, err := t.k8sClient.CoreV1().Services(namespace).Create(bom.service)
			if k8serrors.IsAlreadyExists(err) {
				logr.WithError(err).Warnf("service %s already exists", bom.service.Name)
				return nil
			}
			return err
		}, k8sInteractionBackoff(), func(err error, window time.Duration) {
			logr.WithError(err).Errorf("failed in creating services %s while deploying for training ", bom.service.Name)
			k8sFailureCounter.With(component, "service").Add(1)
		}); err != nil {
			return err
		}

	}

	//create the stateful set
	return backoff.RetryNotify(func() error {
		_, err := t.karClient.ArbV1().XQueueJobs(config.GetPodNamespace()).Create(bom.learnerBOM)
		if k8serrors.IsAlreadyExists(err) {
			logr.WithError(err).Warnf("XQueueJob %s already exists", bom.learnerBOM.Name)
			return nil
		}
		return err
	}, k8sInteractionBackoff(), func(err error, window time.Duration) {
		logr.WithError(err).Errorf("failed in creating XQueueJob %s while deploying for training ", bom.learnerBOM.Name)
		k8sFailureCounter.With(component, "learner").Add(1)
	})

}
