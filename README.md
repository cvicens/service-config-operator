# Works with master branch 
$ operator-sdk version
operator-sdk version: v0.8.0-1-gf7f6440, commit: f7f64400809897a9b21f2c813a4c6e775cc069bc

# prep for go modules
cd $GOPATH
export GO111MODULE=on

# Create new operator
export OPERATOR_NAME="service-config-operator"
export API_VERSION="cloudnative.redhat.com/v1alpha1"

mkdir -p $GOPATH/src/github.com/redhat

cd $GOPATH/src/github.com/redhat

operator-sdk new ${OPERATOR_NAME} --type=go --skip-git-init

cd ./${OPERATOR_NAME}

operator-sdk add api --api-version=${API_VERSION} --kind=ServiceConfig

operator-sdk add controller --api-version=a${API_VERSION} --kind=ServiceConfig

# Modify types
./pkg/apis/app/<version>/<kind>_types.go

# Generate types
operator-sdk generate k8s

# Run this everytime you import a new module
go mod vendor

## List module versions
go list -m -versions gopkg.in/src-d/go-git.v4

# Run locally
export PROJECT_NAME=${OPERATOR_NAME}-tests
oc new-project ${PROJECT_NAME}

oc apply -f deploy/service_account.yaml 
oc apply -f deploy/role.yaml
oc apply -f deploy/role_binding.yaml

oc apply -f deploy/crds/cloudnative_v1alpha1_serviceconfig_crd.yaml
oc apply -f deploy/crds/cloudnative_v1alpha1_serviceconfig_cr.yaml

operator-sdk up local --namespace=${PROJECT_NAME}

# Modules See:
https://github.com/golang/go/wiki/Modules#example