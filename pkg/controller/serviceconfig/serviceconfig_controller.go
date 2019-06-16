package serviceconfig

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"

	cloudnativev1alpha1 "github.com/redhat/service-config-operator/pkg/apis/cloudnative/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	k8s_json "k8s.io/apimachinery/pkg/util/json"
	patch "k8s.io/apimachinery/pkg/util/strategicpatch"
	k8s_yaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	util "github.com/redhat/service-config-operator/pkg/util"

	git "gopkg.in/src-d/go-git.v4"
	plumbing "gopkg.in/src-d/go-git.v4/plumbing"

	//yaml "gopkg.in/yaml.v2"

	logr "github.com/go-logr/logr"

	cmp "github.com/google/go-cmp/cmp"
)

var log = logf.Log.WithName("controller_serviceconfig")

type BaseObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ServiceConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileServiceConfig{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("serviceconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	serviceConfigOk := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log.Info("serviceConfigOk (predicate->UpdateFunc)")
			_, ok := e.ObjectOld.(*cloudnativev1alpha1.ServiceConfig)
			if !ok {
				return false
			}
			newServiceConfig, ok := e.ObjectNew.(*cloudnativev1alpha1.ServiceConfig)
			if !ok {
				return false
			}
			if !newServiceConfig.Spec.Enabled {
				return false
			}

			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			log.Info("serviceConfigOk (predicate->CreateFunc)")
			_, ok := e.Object.(*cloudnativev1alpha1.ServiceConfig)
			if !ok {
				return false
			}

			return true
		},
	}

	// Watch for changes to primary resource ServiceConfig
	err = c.Watch(&source.Kind{Type: &cloudnativev1alpha1.ServiceConfig{}}, &handler.EnqueueRequestForObject{}, serviceConfigOk)
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ServiceConfig
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &cloudnativev1alpha1.ServiceConfig{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileServiceConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileServiceConfig{}

// ReconcileServiceConfig reconciles a ServiceConfig object
type ReconcileServiceConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ServiceConfig object and makes changes based on the state read
// and what is in the ServiceConfig.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileServiceConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ServiceConfig")

	// Fetch the ServiceConfig instance
	instance := &cloudnativev1alpha1.ServiceConfig{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	var descriptorsFolder = util.NVL(instance.Spec.DescriptorsFolder, util.DefaultDescriptorsFolder)
	var descriptorsFolderPath = fmt.Sprintf("%s/%s/", util.GitLocalFilder, descriptorsFolder)

	configMapList := &corev1.ConfigMapList{}
	if err := getAllConfigMaps(r, request, configMapList); err != nil {
		return reconcile.Result{}, err
	}

	if _, err := cloneRepository(instance.Spec.GitUrl, instance.Spec.GitRef); err != nil {
		return reconcile.Result{}, err
	}

	// Apply all ConfigMap and Secret descriptors if needed
	status, err := applyConfigurationDescriptorsInFolder(r, request, descriptorsFolderPath)
	if err != nil {
		return reconcile.Result{}, err
	}

	status.State = "IN_PROGRESS"

	reqLogger.Info("status: " + fmt.Sprintf("%+v", status))
	reqLogger.Info("instance:.Status " + fmt.Sprintf("%+v", instance.Status))
	if !reflect.DeepEqual(instance.Status, *status) {
		reqLogger.Info("status differs")
		instance.Status = *status
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update ServiceConfig status")
			return reconcile.Result{}, err
		}
	}

	// Define a new Pod object
	pod := newPodForCR(instance)

	// Set ServiceConfig instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Pod already exists - don't requeue
	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *cloudnativev1alpha1.ServiceConfig) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

func getAllConfigMaps(r *ReconcileServiceConfig, request reconcile.Request, configmapList *corev1.ConfigMapList) error {
	// Return all configmaps in the request namespace with a label of `app=<name>`
	opts := &client.ListOptions{}
	//opts.SetLabelSelector(fmt.Sprintf("app=%s", request.NamespacedName.Name))
	opts.InNamespace(request.NamespacedName.Namespace)

	ctx := context.TODO()
	err := r.client.List(ctx, opts, configmapList)

	return err
}

func cloneRepository(url string, ref string) (*git.Repository, error) {
	if repo, err := git.PlainOpen(util.GitLocalFilder); err == nil {
		if w, err := repo.Worktree(); err == nil {
			if err := w.Pull(&git.PullOptions{RemoteName: "origin"}); err == nil || err.Error() == "already up-to-date" {
				return repo, nil
			} else {
				os.RemoveAll(util.GitLocalFilder)
				return nil, err
			}
		} else {
			os.RemoveAll(util.GitLocalFilder)
			return nil, err
		}
	} else {
		// Delete just in case
		os.RemoveAll(util.GitLocalFilder)
		// Clone
		repo, err := git.PlainClone(util.GitLocalFilder, false, &git.CloneOptions{
			URL:               url,
			ReferenceName:     plumbing.ReferenceName("refs/heads/" + ref),
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		})
		return repo, err
	}
}

func applyConfigurationDescriptorsInFolder(r *ReconcileServiceConfig, request reconcile.Request, folder string) (*cloudnativev1alpha1.ServiceConfigStatus, error) {
	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	status := &cloudnativev1alpha1.ServiceConfigStatus{
		ConfigMapNamesOk:  []string{},
		SecretNamesOk:     []string{},
		ConfigMapNamesErr: []string{},
		SecretNamesErr:    []string{},
	}

	if files, err := ioutil.ReadDir(folder); err == nil {
		for _, f := range files {
			logger.Info("===================== Current file: " + folder + f.Name() + " =====================")
			if b, err := ioutil.ReadFile(folder + f.Name()); err == nil {
				baseObject := &BaseObject{}
				dec := k8s_yaml.NewYAMLOrJSONDecoder(bytes.NewReader(b), 1000)
				if err := dec.Decode(&baseObject); err == nil {
					switch kind := baseObject.Kind; kind {
					case "ConfigMap":
						if err := handleConfigMap(r, logger, request.Namespace, b); err == nil {
							status.ConfigMapNamesOk = append(status.ConfigMapNamesOk, baseObject.Name)
							logger.Info("handleConfigMap.status ok: " + fmt.Sprintf("%v", status.ConfigMapNamesOk))
						} else {
							status.ConfigMapNamesErr = append(status.ConfigMapNamesErr, baseObject.Name)
							logger.Info("handleConfigMap.status err: " + fmt.Sprintf("%v", status.ConfigMapNamesErr))
						}
					case "Secret":
						if err := handleSecret(r, logger, request.Namespace, b); err == nil {
							status.SecretNamesOk = append(status.SecretNamesOk, baseObject.Name)
						} else {
							status.SecretNamesErr = append(status.SecretNamesErr, baseObject.Name)
						}
					default:
						logger.Info("===== isOther =====" + kind)
					}
				} else {
					logger.Info("Unmarshall BaseObject error: " + err.Error())
				}
			} else {
				logger.Info("ReadFile error: " + err.Error())
			}
		}
	} else {
		logger.Info("ReadDir error: " + err.Error())
		return nil, err
	}

	return status, nil
}

func diff(original, modified runtime.Object) ([]byte, error) {
	origBytes, err := k8s_json.Marshal(original)
	if err != nil {
		return nil, err
	}

	modBytes, err := k8s_json.Marshal(modified)
	if err != nil {
		return nil, err
	}

	return patch.CreateTwoWayMergePatch(origBytes, modBytes, original)
}

func calculateMergePatchBytes(original, modified runtime.Object, dataStruct interface{}) ([]byte, error) {
	logger := log.WithValues()

	patchBytes, err := diff(original, modified)
	if err != nil {
		return nil, err
	}
	origBytes, err := k8s_json.Marshal(original)
	if err != nil {
		return nil, err
	}

	logger.Info(fmt.Sprintf("diff (-want +got):\n%s", patchBytes))

	obj, err := patch.StrategicMergePatch(origBytes, patchBytes, dataStruct)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// Returns calculated merge patch in dataStruct
func calculateMergePatchObject(original, modified runtime.Object, dataStruct interface{}) error {
	mergePatchBytes, err := calculateMergePatchBytes(original, modified, dataStruct)
	if err != nil {
		return err
	}

	if err = k8s_json.Unmarshal(mergePatchBytes, dataStruct); err != nil {
		return err
	}

	return nil
}

func checkIfUpdateConfigMap(fromFile, fromK8s *corev1.ConfigMap) bool {
	logger := log.WithValues("Struct", "ConfigMap")
	type intersection struct {
		Labels      map[string]string
		Annotations map[string]string
		Data        map[string]string
		BinaryData  map[string][]byte
	}

	src := &intersection{fromFile.Labels, fromFile.Annotations, fromFile.Data, fromFile.BinaryData}
	des := &intersection{fromK8s.Labels, fromK8s.Annotations, fromK8s.Data, fromK8s.BinaryData}

	if diff := cmp.Diff(src, des); diff != "" {
		logger.Info(fmt.Sprintf("mismatch (-want +got):\n%s", diff))
	}

	return !cmp.Equal(src, des, nil)
}

func handleConfigMap(r *ReconcileServiceConfig, logger logr.Logger, namespace string, buffer []byte) error {
	logger.Info("===== ConfigMap =====")
	fromK8s := &corev1.ConfigMap{}
	fromFile := &corev1.ConfigMap{}
	fromFile.Namespace = namespace
	dec := k8s_yaml.NewYAMLOrJSONDecoder(bytes.NewReader(buffer), 1000)
	var err error
	if err = dec.Decode(&fromFile); err == nil {
		if err = r.client.Get(context.TODO(), types.NamespacedName{Name: fromFile.Name, Namespace: fromFile.Namespace}, fromK8s); err == nil {
			fromFile.ObjectMeta.ResourceVersion = fromK8s.ObjectMeta.ResourceVersion
			if checkIfUpdateConfigMap(fromFile, fromK8s) {
				mergedPatchObject := &corev1.ConfigMap{}
				patchError := calculateMergePatchObject(fromK8s, fromFile, mergedPatchObject)
				if patchError == nil {
					if err = r.client.Update(context.TODO(), mergedPatchObject); err != nil {
						logger.Info("Update ConfigMap err: " + err.Error())
					}
				} else {
					logger.Info("=======> patchError: " + patchError.Error())
				}
			}
		} else {
			if err = r.client.Create(context.TODO(), fromFile); err != nil {
				logger.Info("Create ConfigMap err: " + err.Error())
			}
		}
	} else {
		logger.Info("Unmarshal ConfigMap err: " + err.Error())
	}

	return err
}

func checkIfUpdateSecret(fromFile, fromK8s *corev1.Secret) bool {
	logger := log.WithValues("Struct", "Secret")
	type intersection struct {
		Labels      map[string]string
		Annotations map[string]string
		Data        map[string][]byte
		StringData  map[string]string
		Type        corev1.SecretType
	}

	src := &intersection{fromFile.Labels, fromFile.Annotations, fromFile.Data, fromFile.StringData, fromFile.Type}
	des := &intersection{fromK8s.Labels, fromK8s.Annotations, fromK8s.Data, fromK8s.StringData, fromK8s.Type}

	if diff := cmp.Diff(src, des); diff != "" {
		logger.Info(fmt.Sprintf("mismatch (-want +got):\n%s", diff))
	}

	return !cmp.Equal(src, des, nil)
}

func handleSecret(r *ReconcileServiceConfig, logger logr.Logger, namespace string, buffer []byte) error {
	logger.Info("===== Secret =====")
	fromK8s := &corev1.Secret{}
	fromFile := &corev1.Secret{}
	fromFile.Namespace = namespace
	dec := k8s_yaml.NewYAMLOrJSONDecoder(bytes.NewReader(buffer), 1000)
	var err error
	if err = dec.Decode(&fromFile); err == nil {
		if err = r.client.Get(context.TODO(), types.NamespacedName{Name: fromFile.Name, Namespace: fromFile.Namespace}, fromK8s); err == nil {
			fromFile.ObjectMeta.ResourceVersion = fromK8s.ObjectMeta.ResourceVersion
			if checkIfUpdateSecret(fromFile, fromK8s) {
				mergedPatchObject := &corev1.Secret{}
				patchError := calculateMergePatchObject(fromK8s, fromFile, mergedPatchObject)
				if patchError == nil {
					logger.Info("Updating with: " + mergedPatchObject.String())
					if err = r.client.Update(context.TODO(), mergedPatchObject); err != nil {
						logger.Info("Update Secret err: " + err.Error())
					}
				} else {
					logger.Info("=======> patchError: " + patchError.Error())
				}
			}
		} else {
			if err = r.client.Create(context.TODO(), fromFile); err != nil {
				logger.Info("Create Secret err: " + err.Error())
			}
		}
	} else {
		logger.Info("Unmarshal Secret err: " + err.Error())
	}

	return err
}
