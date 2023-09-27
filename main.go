package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type ProjectEvent struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func main() {
	// Load the API endpoint from the environment variable
	apiEndpoint, exists := os.LookupEnv("API_ENDPOINT")
	if !exists {
		log.Fatal("API_ENDPOINT not set")
	}

	// Create the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create in-cluster config: %v", err)
	}

	// Create the Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Watch for changes in the Rancher projects
	watchlist := cache.NewListWatchFromClient(
		clientset.RESTClient(),
		"projects.management.cattle.io",
		"",
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchlist,
		&unstructured.Unstructured{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if project, ok := obj.(*unstructured.Unstructured); ok {
					namespace := project.GetNamespace()
					name := project.GetName()
					fmt.Printf("Detected new Rancher project. Namespace: %s, Name: %s\n", namespace, name)
					sendProjectEvent(apiEndpoint, namespace, name)
				}
			},
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	select {}
}

func sendProjectEvent(apiEndpoint, namespace, name string) {
	event := ProjectEvent{
		Namespace: namespace,
		Name:      name,
	}
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal JSON: %v", err)
		return
	}

	resp, err := http.Post(apiEndpoint, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to send event to API: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Received non-OK response from API: %v", resp.Status)
	}
}
