package pod

import (
	"fmt"

	"condenser/internal/store/psm"
	"condenser/internal/utils"
)

// == service: create pod sandbox ==
func (s *PodService) Create(createParameter ServiceCreateModel) (string, error) {
	templateId := utils.NewUlid()

	if err := s.psmHandler.StorePodTemplate(templateId, psm.PodTemplateSpec{
		Name:        createParameter.Name,
		Namespace:   createParameter.Namespace,
		NetworkNS:   createParameter.NetworkNS,
		IPCNS:       createParameter.IPCNS,
		UTSNS:       createParameter.UTSNS,
		UserNS:      createParameter.UserNS,
		Labels:      createParameter.Labels,
		Annotations: createParameter.Annotations,
		Containers:  createParameter.Containers,
	}); err != nil {
		return "", err
	}

	return s.createWithTemplate(templateId, createParameter)
}

// == service: recreate pod sandbox from template ==
func (s *PodService) RecreateFromTemplate(templateId string) (string, error) {
	return s.CreateFromTemplate(templateId, "")
}

// == service: create pod sandbox from template ==
func (s *PodService) CreateFromTemplate(templateId string, nameOverride string) (string, error) {
	template, err := s.psmHandler.GetPodTemplate(templateId)
	if err != nil {
		return "", err
	}

	name := template.Spec.Name
	if nameOverride != "" {
		name = nameOverride
	}

	createParameter := ServiceCreateModel{
		Name:        name,
		Namespace:   template.Spec.Namespace,
		NetworkNS:   template.Spec.NetworkNS,
		IPCNS:       template.Spec.IPCNS,
		UTSNS:       template.Spec.UTSNS,
		UserNS:      template.Spec.UserNS,
		Labels:      template.Spec.Labels,
		Annotations: template.Spec.Annotations,
	}

	return s.createWithTemplate(templateId, createParameter)
}

func (s *PodService) createWithTemplate(templateId string, createParameter ServiceCreateModel) (string, error) {
	if createParameter.Name == "" {
		return "", fmt.Errorf("pod name is required")
	}
	if createParameter.Namespace == "" {
		return "", fmt.Errorf("pod namespace is required")
	}
	if createParameter.UID == "" {
		createParameter.UID = utils.NewUlid()
	}

	if existingId, err := s.psmHandler.GetPodIdByName(createParameter.Name, createParameter.Namespace); err == nil {
		existing, err := s.psmHandler.GetPodById(existingId)
		if err == nil {
			if existing.UID != "" && createParameter.UID != "" && existing.UID == createParameter.UID {
				return existingId, nil
			}
			if existing.UID != "" && createParameter.UID != "" && existing.UID != createParameter.UID {
				if _, err := s.Remove(existingId); err != nil {
					return "", err
				}
			} else {
				return "", fmt.Errorf("pod name already used: %s/%s", createParameter.Namespace, createParameter.Name)
			}
		}
	}

	podId := utils.NewUlid()
	if err := s.psmHandler.StorePod(
		podId,
		templateId,
		createParameter.Name,
		createParameter.Namespace,
		createParameter.UID,
		"created",
		createParameter.NetworkNS,
		createParameter.IPCNS,
		createParameter.UTSNS,
		createParameter.UserNS,
		createParameter.Labels,
		createParameter.Annotations,
	); err != nil {
		return "", err
	}

	return podId, nil
}
