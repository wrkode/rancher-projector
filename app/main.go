package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"log"
	"net/http"
	"os"
)

type ProjectEvent struct {
	Namespace   string            `json:"namespace"`
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
}

var httpClient = &http.Client{}
var debugOutput = false // Set to false to disable debugging output

func main() {
	bearerToken := os.Getenv("BEARER_TOKEN")
	if bearerToken == "" {
		log.Fatal("BEARER_TOKEN is not set or empty")
	}

	rancherFQDN := os.Getenv("RANCHER_FQDN")
	if rancherFQDN == "" {
		log.Fatal("RANCHER_FQDN is not set or empty")
	}

	// Create the in-cluster config
	skipTLSVerify := false
	if os.Getenv("skipTLSVerify") == "true" {
		skipTLSVerify = true
	}

	if skipTLSVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create in-cluster config: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create dynamic client: %v", err)
	}

	// Define the GroupVersionResource for projects.management.cattle.io
	gvr := schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3", // Adjust the version as needed
		Resource: "projects",
	}

	dynamicInterface := dynamicClient.Resource(gvr).Namespace(v1.NamespaceAll)
	watchlist := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return dynamicInterface.List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return dynamicInterface.Watch(context.TODO(), options)
		},
	}

	_, controller := cache.NewInformer(
		watchlist,
		&unstructured.Unstructured{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				handleProjectEvent(obj, "add", rancherFQDN, bearerToken)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				handleProjectEvent(newObj, "update", rancherFQDN, bearerToken)
			},
			DeleteFunc: func(obj interface{}) {
				handleProjectEvent(obj, "delete", rancherFQDN, bearerToken)
			},
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	select {}
}

func handleProjectEvent(obj interface{}, eventType, rancherFQDN, bearerToken string) {
	if project, ok := obj.(*unstructured.Unstructured); ok {
		namespace := project.GetNamespace()
		name := project.GetName()
		annotations := make(map[string]string)

		// Extract annotations
		if meta, ok := project.Object["metadata"].(map[string]interface{}); ok {
			if annos, ok := meta["annotations"].(map[string]interface{}); ok {
				for key, value := range annos {
					strValue, _ := value.(string)
					annotations[key] = strValue
				}
			}
		}

		switch eventType {
		case "add", "update":
			endpoint := fmt.Sprintf("https://%s/k8s/clusters/%s/api/v1/namespaces/kube-system/services/http:rancher-selector-service:8080/proxy/", rancherFQDN, namespace)
			sendProjectEvent(endpoint, namespace, name, annotations, bearerToken, "POST")
		case "delete":
			endpoint := fmt.Sprintf("https://%s/k8s/clusters/%s/api/v1/namespaces/kube-system/services/http:rancher-selector-service:8080/proxy/delete", rancherFQDN, namespace)
			sendProjectEvent(endpoint, namespace, name, annotations, bearerToken, "DELETE")
		}
	}
}

func sendProjectEvent(apiEndpoint, namespace, name string, annotations map[string]string, bearerToken, method string) {
	event := ProjectEvent{
		Namespace:   namespace,
		Name:        name,
		Annotations: annotations,
	}
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal JSON: %v", err)
		return
	}

	// Debugging Output
	if debugOutput {
		log.Println("Sending JSON Data:", string(data))
	}

	req, err := http.NewRequest(method, apiEndpoint, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to create a new request: %v", err)
		return
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to send event to API (%s): %v", apiEndpoint, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Received non-OK response from API (%s): %v", apiEndpoint, resp.Status)
	}
}
