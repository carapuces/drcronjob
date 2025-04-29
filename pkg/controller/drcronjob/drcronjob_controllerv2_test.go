/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package drcronjob

import (
	"fmt"
	"github.com/carapuces/drcronjob/pkg/controller"
	"github.com/carapuces/drcronjob/third_party/robfig/cron/v3"
	drbatchv1 "github.com/carapuces/drcronjobclient/pkg/apis/drcronjob/v1"
	drclientset "github.com/carapuces/drcronjobclient/pkg/generated/clientset/versioned"
	drbatchv1informers "github.com/carapuces/drcronjobclient/pkg/generated/informers/externalversions"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"context"
	apiclient "github.com/carapuces/drcronjob/pkg/util/client"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	shortDead  int64 = 10
	mediumDead int64 = 2 * 60 * 60
	longDead   int64 = 1000000
	noDead     int64 = -12345

	errorSchedule = "obvious error schedule"
	// schedule is hourly on the hour
	onTheHour = "0 * * * ?"
	everyHour = "@every 1h"

	errorTimeZone = "bad timezone"
	newYork       = "America/New_York"
)

// returns a cronJob with some fields filled in.
func drcronJob() drbatchv1.DRCronJob {
	return drbatchv1.DRCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "mycronjob",
			Namespace:         "snazzycats",
			UID:               types.UID("1a2b3c"),
			CreationTimestamp: metav1.Time{Time: justBeforeTheHour()},
		},
		Spec: drbatchv1.DRCronJobSpec{
			Schedule:          "* * * * ?",
			ConcurrencyPolicy: "Allow",
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"a": "b"},
					Annotations: map[string]string{"x": "y"},
				},
				Spec: jobSpec(),
			},
		},
	}
}

func jobSpec() batchv1.JobSpec {
	one := int32(1)
	return batchv1.JobSpec{
		Parallelism: &one,
		Completions: &one,
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{Image: "foo/bar"},
				},
			},
		},
	}
}

func justASecondBeforeTheHour() time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-19T09:59:59Z")
	if err != nil {
		panic("test setup error")
	}
	return T1
}

func justAfterThePriorHour() time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-19T09:01:00Z")
	if err != nil {
		panic("test setup error")
	}
	return T1
}

func justBeforeThePriorHour() time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-19T08:59:00Z")
	if err != nil {
		panic("test setup error")
	}
	return T1
}

func justAfterTheHour() *time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-19T10:01:00Z")
	if err != nil {
		panic("test setup error")
	}
	return &T1
}

func justAfterTheHourInZone(tz string) time.Time {
	location, err := time.LoadLocation(tz)
	if err != nil {
		panic("tz error: " + err.Error())
	}

	T1, err := time.ParseInLocation(time.RFC3339, "2016-05-19T10:01:00Z", location)
	if err != nil {
		panic("test setup error: " + err.Error())
	}
	return T1
}

func justBeforeTheHour() time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-19T09:59:00Z")
	if err != nil {
		panic("test setup error")
	}
	return T1
}

func justBeforeTheNextHour() time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-19T10:59:00Z")
	if err != nil {
		panic("test setup error")
	}
	return T1
}

func weekAfterTheHour() time.Time {
	T1, err := time.Parse(time.RFC3339, "2016-05-26T10:00:00Z")
	if err != nil {
		panic("test setup error")
	}
	return T1
}

func TestUpdateCron(t *testing.T) {
	//lcoalRestConfig, err := apiclient.RestConfig("kubernetes-admin@cluster.local", "/Users/ruibin/GolandProjects/DRPCronJob/kubeconfig/localconfig")
	//localKubeClient := kubernetes.NewForConfigOrDie(lcoalRestConfig)
	//
	//sharedInformers := informers.NewSharedInformerFactory(localKubeClient, controller.NoResyncPeriodFunc())
	//jm, err := NewControllerV2(context.TODO(), sharedInformers.Batch().V1().Jobs(), sharedInformers.Batch().V1().CronJobs(), localKubeClient)
	//if err != nil {
	//	t.Errorf("unexpected error %v", err)
	//	return
	//}
	//jm.now = justASecondBeforeTheHour
	//queue := &fakeQueue{TypedRateLimitingInterface: workqueue.NewTypedRateLimitingQueueWithConfig(
	//	workqueue.DefaultTypedControllerRateLimiter[string](),
	//	workqueue.TypedRateLimitingQueueConfig[string]{
	//		Name: "test-update-cronjob",
	//	},
	//)}
	//jm.queue = queue
	//jm.jobControl = &fakeJobControl{}
	//jm.drCronJobControl = &fakeCJControl{}
	//jm.recorder = record.NewFakeRecorder(10)
	//logger, _ := ktesting.NewTestContext(t)
	//oldCronJob := &batchv1.CronJob{
	//	Spec: batchv1.CronJobSpec{
	//		Schedule: "30 * * * *",
	//		JobTemplate: batchv1.JobTemplateSpec{
	//			ObjectMeta: metav1.ObjectMeta{
	//				Labels:      map[string]string{"a": "b"},
	//				Annotations: map[string]string{"x": "y"},
	//			},
	//			Spec: jobSpec(),
	//		},
	//	},
	//	Status: batchv1.CronJobStatus{
	//		LastScheduleTime: &metav1.Time{Time: justBeforeTheHour()},
	//	}}
	//newCronJob := &batchv1.CronJob{
	//	Spec: batchv1.CronJobSpec{
	//		Schedule: "*/1 * * * *",
	//		JobTemplate: batchv1.JobTemplateSpec{
	//			ObjectMeta: metav1.ObjectMeta{
	//				Labels:      map[string]string{"a": "b"},
	//				Annotations: map[string]string{"x": "y"},
	//			},
	//			Spec: jobSpec(),
	//		},
	//	},
	//	Status: batchv1.CronJobStatus{
	//		LastScheduleTime: &metav1.Time{Time: justBeforeTheHour()},
	//	}}
	//jm.updateCronJob(logger, oldCronJob, newCronJob)
	//expectedDelay := 1*time.Second + nextScheduleDelta
	//fmt.Println(expectedDelay.Seconds())
	//if queue.delay.Seconds() != expectedDelay.Seconds() {
	//	t.Errorf("Expected delay %#v got %#v", expectedDelay, queue.delay.Seconds())
	//}
}

func TestRun(t *testing.T) {
	lcoalRestConfig, err := apiclient.RestConfig("kubernetes-admin@cluster.local", "/Users/ruibin/GolandProjects/DRPCronJob/kubeconfig/localconfig")
	localKubeClient := kubernetes.NewForConfigOrDie(lcoalRestConfig)
	localDRCronClient := drclientset.NewForConfigOrDie(lcoalRestConfig)
	version, err := localDRCronClient.Discovery().ServerVersion()
	if err != nil {
		return
	}
	klog.Infof("version: %v", version)
	gv := localDRCronClient.BatchV1().RESTClient().APIVersion()
	klog.Infof("gv: %v", gv)
	klog.Infof("%v", gv.Version)
	klog.Infof("%v", gv.Group)
	klog.Infof("%v", gv.String())
	obj, err := localDRCronClient.BatchV1().DRCronJobs("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		klog.Infof("%v", obj)
		return
	}
	fmt.Printf("%v", obj)
	drsharedInformers := drbatchv1informers.NewSharedInformerFactory(localDRCronClient, controller.NoResyncPeriodFunc())
	sharedInformers := informers.NewSharedInformerFactory(localKubeClient, controller.NoResyncPeriodFunc())
	jobInformer := sharedInformers.Batch().V1().Jobs()
	drcronJobInformer := drsharedInformers.Batch().V1().DRCronJobs()
	jm, err := NewControllerV2(context.TODO(), jobInformer, drcronJobInformer, localKubeClient, localDRCronClient)
	if err != nil {
		t.Errorf("unexpected error %v", err)
		return
	}
	//nsList, nsErr := jm.kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	//if nsErr != nil {
	//	t.Errorf("unexpected error %v", nsErr)
	//}
	stopCh := make(chan struct{})
	//klog.Infof("nsList %v", nsList.Items[0].Name)
	go jm.Run(context.TODO(), 1)
	go jobInformer.Informer().Run(stopCh)
	go drcronJobInformer.Informer().Run(stopCh)
	<-stopCh
}

func TestCron(t *testing.T) {
	standard, err := cron.ParseStandard("1/10 * * * * *")
	if err != nil {
		fmt.Println(err)
		return
	}
	for {
		time.Sleep(2 * time.Second)
		n := standard.Next(time.Now())
		fmt.Println(n)
	}
}

func TestUpdate(t *testing.T) {
	lcoalRestConfig, err := apiclient.RestConfig("kubernetes-admin@cluster.local", "/Users/ruibin/GolandProjects/DRPCronJob/kubeconfig/localconfig")
	localDRCronClient := drclientset.NewForConfigOrDie(lcoalRestConfig)
	version, err := localDRCronClient.Discovery().ServerVersion()
	if err != nil {
		return
	}
	klog.Infof("version: %v", version)
	gv := localDRCronClient.BatchV1().RESTClient().APIVersion()
	klog.Infof("gv: %v", gv)
	klog.Infof("%v", gv.Version)
	klog.Infof("%v", gv.Group)
	klog.Infof("%v", gv.String())
	obj, err := localDRCronClient.BatchV1().DRCronJobs("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		klog.Infof("%v", obj)
		return
	}

	objupdate, errr := localDRCronClient.BatchV1().DRCronJobs("default").UpdateStatus(context.TODO(), &obj.Items[0], metav1.UpdateOptions{})
	if errr != nil {
		klog.Errorf("update drcronjob:%s,status: %v", objupdate.Name, objupdate.Status)
		klog.Errorf("%v", errr)
	} else {
		klog.Infof("update drcronjob:%s,status: %v", objupdate.Name, objupdate.Status)
	}
}
