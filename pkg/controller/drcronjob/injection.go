/*
Copyright 2016 The Kubernetes Authors.

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
	"context"
	"k8s.io/klog/v2"
	"sync"

	drbatchv1 "drp.inspurcloud.cn/drcronjob/pkg/apis/drcronjob/v1"
	drclientset "drp.inspurcloud.cn/drcronjob/pkg/generated/clientset/versioned"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

// cjControlInterface is an interface that knows how to update CronJob status
// created as an interface to allow testing.
type cjControlInterface interface {
	UpdateStatus(ctx context.Context, cj *drbatchv1.DRCronJob) (*drbatchv1.DRCronJob, error)
	// GetDRCronJob retrieves a CronJob.
	GetDRCronJob(ctx context.Context, namespace, name string) (*drbatchv1.DRCronJob, error)
}

// realCJControl is the default implementation of cjControlInterface.
type realCJControl struct {
	KubeClient drclientset.Interface
}

func (c *realCJControl) GetDRCronJob(ctx context.Context, namespace, name string) (*drbatchv1.DRCronJob, error) {
	return c.KubeClient.BatchV1().DRCronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
}

var _ cjControlInterface = &realCJControl{}

func (c *realCJControl) UpdateStatus(ctx context.Context, cj *drbatchv1.DRCronJob) (*drbatchv1.DRCronJob, error) {
	klog.Errorf("update drcronjob:%s,status: %v", cj.Name, cj.Status)
	return c.KubeClient.BatchV1().DRCronJobs(cj.Namespace).UpdateStatus(ctx, cj, metav1.UpdateOptions{})
}

// fakeCJControl is the default implementation of cjControlInterface.
type fakeCJControl struct {
	CronJob *drbatchv1.DRCronJob
	Updates []drbatchv1.DRCronJob
}

func (c *fakeCJControl) GetDRCronJob(ctx context.Context, namespace, name string) (*drbatchv1.DRCronJob, error) {
	if name == c.CronJob.Name && namespace == c.CronJob.Namespace {
		return c.CronJob, nil
	}
	return nil, errors.NewNotFound(schema.GroupResource{
		Group:    "v1beta1",
		Resource: "cronjobs",
	}, name)
}

var _ cjControlInterface = &fakeCJControl{}

func (c *fakeCJControl) UpdateStatus(ctx context.Context, cj *drbatchv1.DRCronJob) (*drbatchv1.DRCronJob, error) {
	c.Updates = append(c.Updates, *cj)
	return cj, nil
}

// ------------------------------------------------------------------ //

// jobControlInterface is an interface that knows how to add or delete jobs
// created as an interface to allow testing.
type jobControlInterface interface {
	// GetJob retrieves a Job.
	GetJob(namespace, name string) (*batchv1.Job, error)
	// CreateJob creates new Jobs according to the spec.
	CreateJob(namespace string, job *batchv1.Job) (*batchv1.Job, error)
	// DeleteJob deletes the Job identified by name.
	// TODO: delete by UID?
	DeleteJob(namespace string, name string) error
}

// realJobControl is the default implementation of jobControlInterface.
type realJobControl struct {
	KubeClient clientset.Interface
	Recorder   record.EventRecorder
}

var _ jobControlInterface = &realJobControl{}

func (r realJobControl) GetJob(namespace, name string) (*batchv1.Job, error) {
	return r.KubeClient.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (r realJobControl) CreateJob(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	createObj, err := r.KubeClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Failed to create job %s/%s: %v", namespace, job.Name, err)
	}
	return createObj, err
}

func (r realJobControl) DeleteJob(namespace string, name string) error {
	background := metav1.DeletePropagationBackground
	return r.KubeClient.BatchV1().Jobs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{PropagationPolicy: &background})
}

type fakeJobControl struct {
	sync.Mutex
	Job           *batchv1.Job
	Jobs          []batchv1.Job
	DeleteJobName []string
	Err           error
	CreateErr     error
	UpdateJobName []string
	PatchJobName  []string
	Patches       [][]byte
}

var _ jobControlInterface = &fakeJobControl{}

func (f *fakeJobControl) CreateJob(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	f.Lock()
	defer f.Unlock()
	if f.CreateErr != nil {
		return nil, f.CreateErr
	}
	f.Jobs = append(f.Jobs, *job)
	job.UID = "test-uid"
	return job, nil
}

func (f *fakeJobControl) GetJob(namespace, name string) (*batchv1.Job, error) {
	f.Lock()
	defer f.Unlock()
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Job, nil
}

func (f *fakeJobControl) DeleteJob(namespace string, name string) error {
	f.Lock()
	defer f.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.DeleteJobName = append(f.DeleteJobName, name)
	return nil
}
