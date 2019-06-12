package controller

import (
	"github.com/redhat/service-config-operator/pkg/controller/serviceconfig"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, serviceconfig.Add)
}
