package imagerunner

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	knative "knative.dev/serving/pkg/client/clientset/versioned"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

const K8sNameSpace = "spaces"

type HttpServer struct {
	knativeClient *knative.Clientset
	k8sClient     *kubernetes.Clientset
	dockerRegBase string
}

func NewHttpServer(config *config.Config) (*HttpServer, error) {
	// 配置Knative客户端
	kubeconfigPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	kubConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		slog.Error("falied to build kubeconfig", "error", err)

		return nil, fmt.Errorf("failed to build kubeconfig,%w", err)
	}

	knativeClient, err := knative.NewForConfig(kubConfig)
	if err != nil {
		slog.Error("falied to create knative client", "error", err)
		return nil, fmt.Errorf("falied to create knative client,%w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(kubConfig)
	if err != nil {
		slog.Error("falied to create k8s client", "error", err)
		return nil, fmt.Errorf("falied to create k8s client,%w", err)
	}

	return &HttpServer{
		knativeClient: knativeClient,
		k8sClient:     k8sClient,
		dockerRegBase: config.Space.DockerRegBase,
	}, nil
}

func (s *HttpServer) Run(port int) error {
	router := gin.Default()
	router.Use(middleware.Log())

	router.POST("/run", s.runImage)
	router.GET("/:namespace/:name/status/:imageID", s.imageStatus)
	router.GET("/:namespace/:name/logs/:imageID", s.imageLogs)

	return router.Run(fmt.Sprintf(":%d", port))
}

func (s *HttpServer) runImage(c *gin.Context) {
	var request struct {
		ImageID string `json:"image_id" binding:"required"`
		Env     string `json:"env"`
	}

	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	srvName := s.imageIDToServiceName(request.ImageID)
	srv, err := s.knativeClient.ServingV1().Services(K8sNameSpace).
		Get(c.Request.Context(), srvName, metav1.GetOptions{})
	if err == nil {
		slog.Info("service already exists,skip create", err, slog.Any("image_id", request.ImageID),
			slog.Any("service", srv))
		c.JSON(http.StatusOK, nil)
		return
	}

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      srvName,
			Namespace: K8sNameSpace,
		},
		Spec: v1.ServiceSpec{
			ConfigurationSpec: v1.ConfigurationSpec{
				Template: v1.RevisionTemplateSpec{
					Spec: v1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{{
								// TODO: docker registry url + image id
								// Image: "ghcr.io/knative/helloworld-go:latest",
								Image: path.Join(s.dockerRegBase, request.ImageID),
								Ports: []corev1.ContainerPort{{
									ContainerPort: 8080,
								}},
								// TODO:set env
								// Env: environment,
							}},
						},
					},
				},
			},
		},
	}

	service, err = s.knativeClient.ServingV1().Services(K8sNameSpace).
		Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		slog.Error("Failed to create service", "error", err, slog.String("image_id", request.ImageID),
			slog.String("srv_name", srvName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service"})
		return
	}

	slog.Info("service created successfully", slog.Any("service", *service))
	c.JSON(http.StatusOK, gin.H{"message": "Service created successfully"})
}

func (s *HttpServer) imageStatus(c *gin.Context) {
	var resp StatusResponse
	imageID := c.Param("imageID")
	srvName := s.imageIDToServiceName(imageID)
	srv, err := s.knativeClient.ServingV1().Services(K8sNameSpace).
		Get(c.Request.Context(), srvName, metav1.GetOptions{})
	if err != nil {
		slog.Error("get image status failed, cannot get service info", slog.Any("error", err),
			slog.String("srv_name", srvName), slog.Any("image_id", imageID))
		resp.Code = 0
		resp.Message = "failed to get service status"
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if srv.IsFailed() {
		resp.Code = 21
		resp.Message = srv.Status.GetCondition(v1.ServiceConditionReady).Message
		slog.Info("get image status success", slog.Any("resp", resp))
		c.JSON(http.StatusOK, resp)
		return
	}

	if srv.IsReady() {
		labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", imageID)
		podNames, err := s.getPodsByLabelSelector(c.Request.Context(), labelSelector, K8sNameSpace)
		if err != nil {
			slog.Error("get image status failed, cantnot get pods info", slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "message": "unkown service status, failed to get pods"})
			return
		}
		if len(podNames) == 0 {
			resp.Code = 25
			resp.Message = "service sleeping, no running pods"
			slog.Info("get image status success", slog.Any("resp", resp))
			c.JSON(http.StatusOK, resp)
			return
		}

		resp.Code = 23
		resp.Message = "service running"
		slog.Info("get image status success", slog.Any("resp", resp))
		c.JSON(http.StatusOK, resp)
		return
	}

	resp.Code = 0
	resp.Message = "unkown service status"
	slog.Info("get image status failed, service is not ready or failed", slog.Any("resp", resp))
	c.JSON(http.StatusInternalServerError, resp)
}

func (s *HttpServer) imageLogs(c *gin.Context) {
	imageID := c.Param("imageID")
	srvName := s.imageIDToServiceName(imageID)
	labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", srvName)
	podNames, err := s.getPodsByLabelSelector(c.Request.Context(), labelSelector, K8sNameSpace)
	if err != nil {
		slog.Error("failed to read image logs, cantnot get pods info", slog.Any("error", err), slog.String("srv_name", srvName), slog.String("image_id", imageID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get pods info"})
		return
	}
	if len(podNames) == 0 {
		slog.Error("failed to read image logs, no running pods", slog.String("srv_name", srvName), slog.String("image_id", imageID))
		c.JSON(http.StatusNotFound, gin.H{"error": "no running pods, service maybe sleeping"})
		return
	}
	podName := podNames[0]

	logs := s.k8sClient.CoreV1().Pods(K8sNameSpace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "user-container",
		Follow:    true,
	})
	stream, err := logs.Stream(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open stream"})
		return
	}
	defer stream.Close()

	// c.Header("Content-Type", "text/event-stream")
	c.Header("Content-Type", "text/plain")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)

	closeNotify := c.Writer.CloseNotify()

	buf := make([]byte, 1024)
	for {
		select {
		case <-closeNotify:
			// slog.Info("client disconnect from logs api", slog.String("image_id", imageID))
			slog.Info("client disconnect from logs api")
			return
		default:
			n, err := stream.Read(buf)
			if err != nil {
				slog.Error("read pod logs failed", slog.Any("error", err), slog.String("image_id", imageID))
				break
			}

			if n > 0 {
				c.Writer.Write(buf[:n])
				c.Writer.Flush()
				slog.Info("send pod logs", slog.String("image_id", imageID), slog.Int("len", n))
			}
			// c.Writer.WriteString("test messagetest messagetest messagetest messagetest messagetest messagetest messagetest messagetest message")
			c.Writer.Flush()
		}
		time.Sleep(5 * time.Second)
	}
}

func (s *HttpServer) getPodsByLabelSelector(ctx context.Context, labelSelector string, namespace string) ([]string, error) {
	// Get the list of Pods based on the label selector
	pods, err := s.k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         1,
	})
	if err != nil {
		return nil, err
	}

	// Extract the Pod names
	var podNames []string
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}

func (s *HttpServer) imageIDToServiceName(imageID string) string {
	return strings.ReplaceAll(imageID, ":", "-")
}
