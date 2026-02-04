package main

import (
	"context"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var port = 30000

func kubeClient() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func createGameServerPod(
	client *kubernetes.Clientset,
) error {

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "game-server-1",
			Namespace: "game-backend",
			Labels: map[string]string{
				"app":  "game-server",
				"room": "1",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "server",
					Image:           "game-backend_server:latest",
					ImagePullPolicy: corev1.PullNever,
					Ports: []corev1.ContainerPort{
						{ContainerPort: 7777},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(7777),
							},
						},
						InitialDelaySeconds: 1,
						PeriodSeconds:       1,
					},
				},
			},
		},
	}

	_, err := client.CoreV1().
		Pods("game-backend").
		Create(context.Background(), pod, metav1.CreateOptions{})

	return err
}

func createClusterIPService(
	client *kubernetes.Clientset,
) (string, error) {

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "game-server-1",
			Namespace: "game-backend",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"room": "1",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       7777,
					TargetPort: intstr.FromInt(7777),
				},
			},
		},
	}

	created, err := client.CoreV1().
		Services("game-backend").
		Create(context.Background(), svc, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	// DNS name inside the cluster
	address := created.Name

	return address, nil
}


func waitForPodReady(
	client *kubernetes.Clientset,
	podName string,
) error {

	for {
		pod, err := client.CoreV1().
			Pods("game-backend").
			Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady &&
				cond.Status == corev1.ConditionTrue {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func watchAndCleanupGameServer(
	client *kubernetes.Clientset,
	podName string,
	serviceName string,
) {

	watcher, err := client.CoreV1().
		Pods("game-backend").
		Watch(context.Background(), metav1.ListOptions{
			FieldSelector: "metadata.name=" + podName,
		})
	if err != nil {
		log.Printf("watch error: %v", err)
		return
	}

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}

		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			log.Printf("Pod %s finished (%s), cleaning up", podName, pod.Status.Phase)

			// Delete Pod
			_ = client.CoreV1().
				Pods("game-backend").
				Delete(context.Background(), podName, metav1.DeleteOptions{})

			// Delete Service
			_ = client.CoreV1().
				Services("game-backend").
				Delete(context.Background(), serviceName, metav1.DeleteOptions{})

			watcher.Stop()
			return
		}
	}
}