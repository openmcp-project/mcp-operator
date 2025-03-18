package releasechannel

import (
	"context"
	"slices"
	"time"

	cpoev1beta1 "github.com/openmcp-project/control-plane-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

const interval = 15 * time.Minute

type ReleasechannelRunnable struct {
	crateClient client.Client
	coreClient  client.Client
}

func NewReleasechannelRunnable(crateClient client.Client, coreClient client.Client) ReleasechannelRunnable {
	return ReleasechannelRunnable{
		crateClient: crateClient,
		coreClient:  coreClient,
	}
}

func (r *ReleasechannelRunnable) NeedLeaderElection() bool {
	return true
}

func (r *ReleasechannelRunnable) Start(ctx context.Context) error {
	ch := time.Tick(interval)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ch:
			err := r.loop(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (r *ReleasechannelRunnable) loop(ctx context.Context) error {
	log := log.FromContext(ctx)

	// Get a list of all managedComponents in the crate cluster
	currentManagedComponentList := v1alpha1.ManagedComponentList{}
	err := r.crateClient.List(ctx, &currentManagedComponentList)
	if err != nil {
		return err
	}

	// Get a list of all releasechannels in the core cluster
	releasechannelList := cpoev1beta1.ReleaseChannelList{}
	err = r.coreClient.List(ctx, &releasechannelList)
	if err != nil {
		return err
	}

	// Flat the components of all releasechannels
	releasechannelComponents := flatAndRemoveDuplicatesReleasechannelComponents(releasechannelList.Items)

	for _, managedcomponent := range currentManagedComponentList.Items {
		// check if the managedComponent is still in any of the releasechannels
		contains := slices.ContainsFunc(releasechannelComponents, func(component cpoev1beta1.Component) bool {
			return component.Name == managedcomponent.Name
		})

		// the managed component is not in any releasechannel anymore, delete it
		if !contains {
			err := r.crateClient.Delete(ctx, &managedcomponent)
			if err != nil {
				log.Error(err, "Failed to delete managedComponent", "name", managedcomponent.Name)
			}
		}
	}

outer:
	for _, component := range releasechannelComponents {
		versions := make([]string, 0, len(component.Versions))
		for _, version := range component.Versions {
			versions = append(versions, version.Version)
		}

		// check wether the crate cluster already has a managedComponent for this component
		for _, managedComponent := range currentManagedComponentList.Items {
			if managedComponent.Name == component.Name {
				managedComponent.Status.Versions = versions
				// The managedComponent is inside the releasechannel Update the managedComponent status
				err := r.crateClient.Status().Update(ctx, &managedComponent)
				if err != nil {
					log.Error(err, "Failed to update managedComponent status", "name", &managedComponent.Name)
				}
				continue outer
			}
		}

		newMc := v1alpha1.ManagedComponent{
			ObjectMeta: metav1.ObjectMeta{
				Name: component.Name,
			},
			Status: v1alpha1.ManagedComponentStatus{
				Versions: versions,
			},
		}

		err := r.crateClient.Create(ctx, &newMc)
		if err != nil {
			log.Error(err, "Failed to create managedComponent", "name", component.Name)
		}

		err = r.crateClient.Status().Update(ctx, &newMc)
		if err != nil {
			log.Error(err, "Failed to update managedComponent status", "name", component.Name)
		}
	}

	return nil
}

func flatAndRemoveDuplicatesReleasechannelComponents(releasechannels []cpoev1beta1.ReleaseChannel) []cpoev1beta1.Component {
	releasechannelComponents := make([]cpoev1beta1.Component, 0)
	for _, releasechannel := range releasechannels {
		// Check if there are components in multiple releasechannels
		for _, component := range releasechannel.Status.Components {
			contains := false
			for _, c := range releasechannelComponents {
				// If the already added component has the same name, append the versions
				if c.Name == component.Name {
					contains = true
					c.Versions = append(c.Versions, component.Versions...)
				}
			}
			// If there is no component with the same name, add the component
			if !contains {
				releasechannelComponents = append(releasechannelComponents, component)
			}
		}
	}

	return releasechannelComponents
}
