package learner

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
func CreateQJPodSpec(containers []v1core.Container, volumes []v1core.Volume, labels map[string]string, nodeSelector map[string]string, learnerName string) v1core.PodTemplateSpec {
	queueJobName := "xqueuejob.arbitrator.k8s.io"
	labels["service"] = "dlaas-learner" //label that denies ingress/egress
	labels[queueJobName] = learnerName
	imagePullSecret := viper.GetString(config.LearnerImagePullSecretKey)
	automountSeviceToken := false
	return v1core.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
			Annotations: map[string]string{
				"scheduler.alpha.kubernetes.io/tolerations": `[ { "key": "dedicated", "operator": "Equal", "value": "gpu-task" } ]`,
				"scheduler.alpha.kubernetes.io/nvidiaGPU":   `{ "AllocationPriority": "Dense" }`,
			},
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
			Tolerations: []v1core.Toleration{
				v1core.Toleration{
					Key:      "dedicated",
					Operator: v1core.TolerationOpEqual,
					Value:    "gpu-task",
					Effect:   v1core.TaintEffectNoSchedule,
				},
			},
			// For FfDL we don't need any constraint on GPU type.
			// NodeSelector:                 nodeSelector,
			AutomountServiceAccountToken: &automountSeviceToken,
		},
	}
}

//CreatePodSpec ...
func CreatePodSpec(containers []v1core.Container, volumes []v1core.Volume, labels map[string]string, nodeSelector map[string]string, imagePullSecret string) v1core.PodTemplateSpec {
	labels["service"] = "dlaas-learner" //label that denies ingress/egress
	automountSeviceToken := false
	return v1core.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
			Annotations: map[string]string{
				"scheduler.alpha.kubernetes.io/tolerations": `[ { "key": "dedicated", "operator": "Equal", "value": "gpu-task" } ]`,
				"scheduler.alpha.kubernetes.io/nvidiaGPU":   `{ "AllocationPriority": "Dense" }`,
			},
		},
		Spec: v1core.PodSpec{
			Containers: containers,
			Volumes:    volumes,
			ImagePullSecrets: []v1core.LocalObjectReference{
				v1core.LocalObjectReference{
					Name: imagePullSecret,
				},
			},
			Tolerations: []v1core.Toleration{
				v1core.Toleration{
					Key:      "dedicated",
					Operator: v1core.TolerationOpEqual,
					Value:    "gpu-task",
					Effect:   v1core.TaintEffectNoSchedule,
				},
			},
			// For FfDL we don't need any constraint on GPU type.
			// NodeSelector:                 nodeSelector,
			AutomountServiceAccountToken: &automountSeviceToken,
		},
	}
}

//CreateQJSpecForLearner ...
func CreateQJStatefulSetSpecForLearner(name, servicename string, replicas int, podTemplateSpec v1core.PodTemplateSpec, trainingId string, userId string) *arbv1.XQueueJob {
	//var replicaCount = int32(replicas)
	//revisionHistoryLimit := int32(0) //https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#clean-up-policy
	queueJobName := "xqueuejob.arbitrator.k8s.io"

	ssSpec := CreateStatefulSetSpecForLearner(name, servicename, replicas, podTemplateSpec)

	data, _ := json2.Marshal(*ssSpec)
	/*if err != nil {
		logr.Errorf("Encoding podTemplate failed %+v %+v", podTemplate, err)
	}*/

	rawExt := runtime.RawExtension{Raw: json2.RawMessage(data)}

	var minAvl int32

	minAvl = 1

	aggrResources := arbv1.XQueueJobResourceList{
		Items: []arbv1.XQueueJobResource{
			arbv1.XQueueJobResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: config.GetPodNamespace(),
					Labels:    map[string]string{queueJobName: name},
				},
				Replicas:     int32(replicas),
				MinAvailable: &minAvl,
				Priority:     float64(50000000),
				Type:         arbv1.ResourceTypeStatefulSet,
				Template:     rawExt,
			},
		},
	}

	qjSpec := &arbv1.XQueueJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.GetPodNamespace(),
			Labels: map[string]string{
				"app":     name,
				"service": servicename,
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

	return qjSpec

	/*return &v1beta1.StatefulSet{

		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: podTemplateSpec.Labels,
		},
		Spec: v1beta1.StatefulSetSpec{
			ServiceName:          servicename,
			Replicas:             &replicaCount,
			Template:             podTemplateSpec,
			RevisionHistoryLimit: &revisionHistoryLimit, //we never rollback these
			//PodManagementPolicy: v1beta1.ParallelPodManagement, //using parallel pod management in stateful sets to ignore the order. not sure if this will affect the helper pod since any pod in learner can come up now
		},
	}*/
}

//CreateQJSpecForLearner ...
func CreateQJSpecForLearner(name, servicename string, replicas int, podTemplateSpec v1core.PodTemplateSpec, trainingId string, userId string) *arbv1.XQueueJob {
	//var replicaCount = int32(replicas)
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

	var minAvl int32

	minAvl = 1

	aggrResources := arbv1.XQueueJobResourceList{
		Items: []arbv1.XQueueJobResource{
			arbv1.XQueueJobResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: config.GetPodNamespace(),
					Labels:    map[string]string{queueJobName: name},
				},
				Replicas:     int32(replicas),
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
				"service": servicename,
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

	/*return &v1beta1.StatefulSet{

		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: podTemplateSpec.Labels,
		},
		Spec: v1beta1.StatefulSetSpec{
			ServiceName:          servicename,
			Replicas:             &replicaCount,
			Template:             podTemplateSpec,
			RevisionHistoryLimit: &revisionHistoryLimit, //we never rollback these
			//PodManagementPolicy: v1beta1.ParallelPodManagement, //using parallel pod management in stateful sets to ignore the order. not sure if this will affect the helper pod since any pod in learner can come up now
		},
	}*/
}

//CreateStatefulSetSpecForLearner ...
func CreateStatefulSetSpecForLearner(name, servicename string, replicas int, podTemplateSpec v1core.PodTemplateSpec) *v1beta1.StatefulSet {
	var replicaCount = int32(replicas)
	revisionHistoryLimit := int32(0) //https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#clean-up-policy

	return &v1beta1.StatefulSet{

		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: podTemplateSpec.Labels,
		},
		Spec: v1beta1.StatefulSetSpec{
			ServiceName:          servicename,
			Replicas:             &replicaCount,
			Template:             podTemplateSpec,
			RevisionHistoryLimit: &revisionHistoryLimit, //we never rollback these
			//PodManagementPolicy: v1beta1.ParallelPodManagement, //using parallel pod management in stateful sets to ignore the order. not sure if this will affect the helper pod since any pod in learner can come up now
		},
	}
}

//CreateServiceSpec ... this service will govern the statefulset
func CreateServiceSpec(name string, trainingID string) *v1core.Service {

	return &v1core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"training_id": trainingID,
			},
		},
		Spec: v1core.ServiceSpec{
			Selector: map[string]string{"training_id": trainingID},
			Ports: []v1core.ServicePort{
				v1core.ServicePort{
					Name:     "ssh",
					Protocol: v1core.ProtocolTCP,
					Port:     22,
				},
				v1core.ServicePort{
					Name:     "tf-distributed",
					Protocol: v1core.ProtocolTCP,
					Port:     2222,
				},
			},
			ClusterIP: "None",
		},
	}
}
