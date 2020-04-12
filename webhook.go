package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var reqLabel = map[string]string{
	"team": "ops",
}

//WebHookServer listen to admission requests and serve responses
type WebHookServer struct {
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func updLabel(target map[string]string, added map[string]string) (patch []patchOperation) {
	values := make(map[string]string)
	for key, value := range added {
		if target == nil || target[key] == "" {
			values[key] = value
		}
	}
	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/metadata/labels",
		Value: values,
	})
	return patch
}
func createPatch(availableLabel map[string]string, label map[string]string) ([]byte, error) {
	var patch []patchOperation

	patch = append(patch, updLabel(availableLabel, label)...)

	return json.Marshal(patch)
}
func (ws *WebHookServer) validate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {

	raw := ar.Request.Object.Raw
	glog.Infof("VALIDATION:AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		ar.Request.Kind, ar.Request.Namespace, ar.Request.Name, ar.Request.UID, ar.Request.Operation, ar.Request.UserInfo)
	pod := v1.Pod{}
	deployment := appsv1.Deployment{}

	if err := json.Unmarshal(raw, &pod); err != nil {
		glog.Error("error deserializing pods")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}

	}
	if err := json.Unmarshal(raw, &deployment); err != nil {
		glog.Error("error deserializing deployments")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}
	if pod.ObjectMeta.Labels["team"] == reqLabel["team"] {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
	if deployment.Labels["team"] == reqLabel["team"] {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
	fmt.Printf("VALIDATION:This is %v value with lables \n", pod.ObjectMeta.Labels)
	fmt.Printf("VALIDATION:This is %v value with lables \n", deployment.Labels)

	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: "This label 'team' is not allowed !",
		},
	}
}

func (ws *WebHookServer) mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	var availableLabel map[string]string
	rk := ar.Request.Kind
	raw := ar.Request.Object.Raw
	pod := v1.Pod{}
	deployment := appsv1.Deployment{}

	glog.Infof("MUTATION:AdmissionReview for Kind=%v, Namespace=%v Name=%v UID=%v patchOperation=%v UserInfo=%v",
		ar.Request.Kind, ar.Request.Namespace, ar.Request.Name, ar.Request.UID, ar.Request.Operation, ar.Request.UserInfo)

	switch rk.Kind {
	case "Pod":
		if err := json.Unmarshal(raw, &pod); err != nil {
			glog.Error("error deserializing pod")
			return &v1beta1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		availableLabel = pod.ObjectMeta.Labels
	case "Deployment":
		if err := json.Unmarshal(raw, &deployment); err != nil {
			return &v1beta1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		availableLabel = deployment.Labels
	}
	//if pod.ObjectMeta.Labels["team"] == reqLabel["team"] || deployment.Labels["team"] == reqLabel["team"] {
	//	return &v1beta1.AdmissionResponse{
	//		Allowed: true,
	//	}
	//}
	pBytes, err := createPatch(availableLabel, reqLabel)
	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   pBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (ws *WebHookServer) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	var admResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if err := json.Unmarshal(body, &ar); err != nil {
		glog.Error("incorrect body")
		http.Error(w, "incorrect body", http.StatusBadRequest)
	}
	glog.Info("Received request")
	fmt.Println(r.URL.Path)
	if r.URL.Path == "/mutate" {
		admResponse = ws.mutate(&ar)
		fmt.Printf("MUTATION:Response Allowed: %v \n", admResponse.Allowed)
	}
	if r.URL.Path == "/validate" {
		admResponse = ws.validate(&ar)
		fmt.Printf("VALIDATION:Response Allowed: %v \n", admResponse.Allowed)
	}
	admReview := v1beta1.AdmissionReview{}
	if admResponse != nil {
		admReview.Response = admResponse
		if ar.Request != nil {
			admReview.Response.UID = ar.Request.UID
		}
	}
	resp, err := json.Marshal(admReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	glog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
