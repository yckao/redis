package controller

import (
	"fmt"
	"reflect"

	"github.com/appscode/go/log"
	"github.com/appscode/kutil"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/apimachinery/pkg/eventer"
	"github.com/kubedb/redis/pkg/validator"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) create(redis *api.Redis) error {
	if err := validator.ValidateRedis(c.Client, redis, c.opt.Docker); err != nil {
		log.Errorln(err)
		c.recorder.Event(
			redis.ObjectReference(),
			core.EventTypeWarning,
			eventer.EventReasonInvalid,
			err.Error(),
		)
		return nil // user error so just record error and don't retry.
	}

	if redis.Status.CreationTime == nil {
		mc, _, err := util.PatchRedis(c.ExtClient, redis, func(in *api.Redis) *api.Redis {
			t := metav1.Now()
			in.Status.CreationTime = &t
			in.Status.Phase = api.DatabasePhaseCreating
			return in
		})
		if err != nil {
			c.recorder.Eventf(
				redis.ObjectReference(),
				core.EventTypeWarning,
				eventer.EventReasonFailedToUpdate,
				err.Error(),
			)
			return err
		}
		redis.Status = mc.Status
	}

	// Dynamic Defaulting
	// Assign Default Monitoring Port
	if err := c.setMonitoringPort(redis); err != nil {
		return err
	}

	// Check DormantDatabase
	// It can be used as resumed
	if err := c.matchDormantDatabase(redis); err != nil {
		return err
	}

	// create Governing Service
	governingService := c.opt.GoverningService
	if err := c.CreateGoverningService(governingService, redis.Namespace); err != nil {
		c.recorder.Eventf(
			redis.ObjectReference(),
			core.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			`Failed to create Service: "%v". Reason: %v`,
			governingService,
			err,
		)
		return err
	}

	// ensure database Service
	ok1, er1 := c.ensureService(redis)
	if er1 != nil {
		return er1
	}

	// ensure database Deployment
	ok2, er2 := c.ensureStatefulSet(redis)
	if er2 != nil {
		return er2
	}

	if ok1 == kutil.VerbCreated && ok2 == kutil.VerbCreated {
		c.recorder.Event(
			redis.ObjectReference(),
			core.EventTypeNormal,
			eventer.EventReasonSuccessful,
			"Successfully created Redis",
		)
	} else if ok1 == kutil.VerbPatched || ok2 == kutil.VerbPatched {
		c.recorder.Event(
			redis.ObjectReference(),
			core.EventTypeNormal,
			eventer.EventReasonSuccessful,
			"Successfully patched Redis",
		)
	}

	if err := c.manageMonitor(redis); err != nil {
		c.recorder.Eventf(
			redis.ObjectReference(),
			core.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			"Failed to manage monitoring system. Reason: %v",
			err,
		)
		log.Errorln(err)
		return nil
	}
	return nil
}

func (c *Controller) setMonitoringPort(redis *api.Redis) error {
	if redis.Spec.Monitor != nil &&
		redis.Spec.Monitor.Prometheus != nil {
		if redis.Spec.Monitor.Prometheus.Port == 0 {
			mc, _, err := util.PatchRedis(c.ExtClient, redis, func(in *api.Redis) *api.Redis {
				in.Spec.Monitor.Prometheus.Port = api.PrometheusExporterPortNumber
				return in
			})
			if err != nil {
				c.recorder.Eventf(
					redis.ObjectReference(),
					core.EventTypeWarning,
					eventer.EventReasonFailedToUpdate,
					err.Error(),
				)
				return err
			}
			redis.Spec = mc.Spec
		}
	}
	return nil
}

func (c *Controller) matchDormantDatabase(redis *api.Redis) error {
	// Check if DormantDatabase exists or not
	dormantDb, err := c.ExtClient.DormantDatabases(redis.Namespace).Get(redis.Name, metav1.GetOptions{})
	if err != nil {
		if !kerr.IsNotFound(err) {
			c.recorder.Eventf(
				redis.ObjectReference(),
				core.EventTypeWarning,
				eventer.EventReasonFailedToGet,
				`Fail to get DormantDatabase: "%v". Reason: %v`,
				redis.Name,
				err,
			)
			return err
		}
		return nil
	}

	var sendEvent = func(message string, args ...interface{}) error {
		c.recorder.Eventf(
			redis.ObjectReference(),
			core.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			message,
			args,
		)
		return fmt.Errorf(message, args)
	}

	// Check DatabaseKind
	if dormantDb.Labels[api.LabelDatabaseKind] != api.ResourceKindRedis {
		return sendEvent(fmt.Sprintf(`Invalid Redis: "%v". Exists DormantDatabase "%v" of different Kind`, redis.Name, dormantDb.Name))
	}

	// Check Origin Spec
	drmnOriginSpec := dormantDb.Spec.Origin.Spec.Redis
	originalSpec := redis.Spec

	if !reflect.DeepEqual(drmnOriginSpec, &originalSpec) {
		return sendEvent("Redis spec mismatches with OriginSpec in DormantDatabases")
	}

	return util.DeleteDormantDatabase(c.ExtClient, dormantDb.ObjectMeta)
}

func (c *Controller) pause(redis *api.Redis) error {
	//if redis.Spec.DoNotPause {
	//	c.recorder.Eventf(
	//		redis.ObjectReference(),
	//		core.EventTypeWarning,
	//		eventer.EventReasonFailedToPause,
	//		`Redis "%v" is locked.`,
	//		redis.Name,
	//	)
	//
	//	if err := c.reCreateRedis(redis); err != nil {
	//		c.recorder.Eventf(
	//			redis.ObjectReference(),
	//			core.EventTypeWarning,
	//			eventer.EventReasonFailedToCreate,
	//			`Failed to recreate Redis: "%v". Reason: %v`,
	//			redis.Name,
	//			err,
	//		)
	//		return err
	//	}
	//	return nil
	//}

	if _, err := c.createDormantDatabase(redis); err != nil {
		c.recorder.Eventf(
			redis.ObjectReference(),
			core.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			`Failed to create DormantDatabase: "%v". Reason: %v`,
			redis.Name,
			err,
		)
		return err
	}
	c.recorder.Eventf(
		redis.ObjectReference(),
		core.EventTypeNormal,
		eventer.EventReasonSuccessfulCreate,
		`Successfully created DormantDatabase: "%v"`,
		redis.Name,
	)

	if redis.Spec.Monitor != nil {
		if _, err := c.deleteMonitor(redis); err != nil {
			c.recorder.Eventf(
				redis.ObjectReference(),
				core.EventTypeWarning,
				eventer.EventReasonFailedToDelete,
				"Failed to delete monitoring system. Reason: %v",
				err,
			)
			log.Errorln(err)
			return nil
		}
	}
	return nil
}