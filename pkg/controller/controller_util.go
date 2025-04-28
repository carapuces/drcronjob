package controller

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"time"
)

var (
	KeyFunc           = cache.DeletionHandlingMetaNamespaceKeyFunc
	podPhaseToOrdinal = map[v1.PodPhase]int{v1.PodPending: 0, v1.PodUnknown: 1, v1.PodRunning: 2}
)

func NoResyncPeriodFunc() time.Duration {
	return 0
}
