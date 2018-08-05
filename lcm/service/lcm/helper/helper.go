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

package helper

import (
	json2 "encoding/json"
	"github.com/IBM/FfDL/commons/config"
	arbv1 "github.com/kubernetes-incubator/kube-arbitrator/pkg/apis/v1alpha1"
	"github.com/spf13/viper"
	v1beta1 "k8s.io/api/apps/v1beta1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//CreateQJPodSpec ...
func CreateQJHelperPodSpec(containers []v1core.Container, volumes []v1core.Volume, labels map[string]string, helperName string) v1core.PodTemplateSpec {
	queueJobName := "xqueuejob.arbitrator.k8s.io"
	labels[queueJobName] = helperName
	labels["service"] = "dlaas-lhelper" //controls ingress/egress
	imagePullSecret := viper.GetString(config.LearnerImagePullSecretKey)
	automountSeviceToken := false
	return v1core.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: v1core.PodSpec{
			SchedulerName: "bsa-gang-scheduler",
			Containers:    containers,
			Volumes:       volumes,
			ImagePullSecrets: []v1core.LocalObjectReference{
				v1core.LocalObjectReference{
					Name: imagePullSecret,
				},
			},
			AutomountServiceAccountToken: &automountSeviceToken,
		},
	}
}

//CreateQJForHelper ...
func CreateQJForHelper(name string, podTemplateSpec v1core.PodTemplateSpec, trainingId string, userId string) *arbv1.XQueueJob {

	//revisionHistoryLimit := int32(0) //https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#clean-up-policy

	queueJobName := "xqueuejob.arbitrator.k8s.io"

	podTemplate := v1core.PodTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				queueJobName:  name,
				"training_id": trainingId,
				"user_id":     userId,
			},
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1", Kind: "PodTemplate",
		},
		Template: podTemplateSpec,
	}

	data, _ := json2.Marshal(podTemplate)
	/*if err != nil {
		logr.Errorf("Encoding podTemplate failed %+v %+v", podTemplate, err)
	}*/

	rawExt := runtime.RawExtension{Raw: json2.RawMessage(data)}

	var minAvl, replicas int32

	minAvl = 1
	replicas = 1

	aggrResources := arbv1.XQueueJobResourceList{
		Items: []arbv1.XQueueJobResource{
			arbv1.XQueueJobResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: config.GetPodNamespace(),
					Labels:    map[string]string{queueJobName: name},
				},
				Replicas:     replicas,
				MinAvailable: &minAvl,
				Priority:     float64(50000000),
				Type:         arbv1.ResourceTypePod,
				Template:     rawExt,
			},
		},
	}

	ssSpec := &arbv1.XQueueJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.GetPodNamespace(),
			Labels: map[string]string{
				"app":     name,
				"service": "dlaas-helper",
			},
		},
		Spec: arbv1.XQueueJobSpec{
			Priority: 50000000,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					queueJobName: name,
				},
			},
			SchedSpec: arbv1.SchedulingSpecTemplate{
				MinAvailable: int(1),
			},
			AggrResources: aggrResources,
		},
	}

	return ssSpec
}

//CreatePodSpec ...
func CreatePodSpec(containers []v1core.Container, volumes []v1core.Volume, labels map[string]string) v1core.PodTemplateSpec {
	labels["service"] = "dlaas-lhelper" //controls ingress/egress
	imagePullSecret := viper.GetString(config.LearnerImagePullSecretKey)
	automountSeviceToken := false
	return v1core.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: v1core.PodSpec{
			Containers: containers,
			Volumes:    volumes,
			ImagePullSecrets: []v1core.LocalObjectReference{
				v1core.LocalObjectReference{
					Name: imagePullSecret,
				},
			},
			AutomountServiceAccountToken: &automountSeviceToken,
		},
	}
}

//CreateDeploymentForHelper ...
func CreateDeploymentForHelper(name string, podTemplateSpec v1core.PodTemplateSpec) *v1beta1.Deployment {

	revisionHistoryLimit := int32(0) //https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#clean-up-policy

	//TODO consider this as well https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#progress-deadline-seconds
	//but not sure if we can nicely revert back
	return &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.DeploymentSpec{
			Template:             podTemplateSpec,
			RevisionHistoryLimit: &revisionHistoryLimit, //we never rollback these
		},
	}
}
