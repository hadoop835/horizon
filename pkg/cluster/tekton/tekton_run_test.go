package tekton

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"g.hz.netease.com/horizon/pkg/config/tekton"

	"github.com/gorilla/mux"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	fakedtekton "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeddynamic "k8s.io/client-go/dynamic/fake"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var tektonConfig = &tekton.Tekton{
	Server:    "",
	Namespace: "tekton",
}

func TestTekton_StopPipelineRun(t1 *testing.T) {
	pr1 := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1-1-1",
			Namespace: "tekton",
			Labels: map[string]string{
				labelKeyApplication:   "test-app",
				labelKeyCluster:       "test-cluster",
				"tekton.dev/pipeline": "default",
			},
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
						Reason: string(v1beta1.PipelineRunReasonSuccessful),
					},
				},
			},
		},
	}
	pr2 := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test2-2-1",
			Namespace: "tekton",
			Labels: map[string]string{
				labelKeyApplication:   "test-app",
				labelKeyCluster:       "test-cluster",
				"tekton.dev/pipeline": "default",
			},
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
						Reason: string(v1beta1.PipelineRunReasonRunning),
					},
				},
			},
		},
	}
	type fields struct {
		TektonConfig *tekton.Tekton
	}
	type args struct {
		ctx           context.Context
		cluster       string
		clusterID     uint
		pipelinerunID uint
	}
	tests := []struct {
		name         string
		fields       fields
		pipelineRuns []runtime.Object
		args         args
		wantErr      bool
	}{
		{
			name: "stop pipelineRun normal",
			fields: fields{
				TektonConfig: tektonConfig,
			},
			pipelineRuns: []runtime.Object{pr1},
			args: args{
				ctx:           context.Background(),
				cluster:       "test1",
				clusterID:     1,
				pipelinerunID: 1,
			},
			wantErr: false,
		},
		{
			name: "stop pipelineRun normal",
			fields: fields{
				TektonConfig: tektonConfig,
			},
			pipelineRuns: []runtime.Object{pr2},
			args: args{
				ctx:           context.Background(),
				cluster:       "test2",
				clusterID:     2,
				pipelinerunID: 1,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Tekton{
				namespace: tektonConfig.Namespace,
				client: &Client{
					Tekton:  fakedtekton.NewSimpleClientset(tt.pipelineRuns...),
					Dynamic: fakeddynamic.NewSimpleDynamicClient(runtime.NewScheme()),
				},
			}
			if err := t.StopPipelineRun(tt.args.ctx, tt.args.cluster, tt.args.clusterID,
				tt.args.pipelinerunID); (err != nil) != tt.wantErr {
				t1.Errorf("StopPipelineRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type MockTektonController struct {
	R *mux.Router
}

func errResponse(err error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(err.Error()))
}

func NewMockTektonController() *MockTektonController {
	r := mux.NewRouter()
	c := &MockTektonController{
		R: r,
	}
	r.Methods(http.MethodPost).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errResponse(err, w)
			return
		}
		var pr PipelineRun
		if err := json.Unmarshal(data, &pr); err != nil {
			errResponse(err, w)
			return
		}
		if pr.Application == "app" {
			resp := map[string]string{
				"eventID": "1234",
			}
			b, err := json.Marshal(resp)
			if err != nil {
				errResponse(err, w)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(b)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	return c
}

func TestMain(m *testing.M) {
	c := NewMockTektonController()
	s := httptest.NewServer(http.HandlerFunc(c.R.ServeHTTP))
	tektonConfig.Server = s.Listener.Addr().String()
	os.Exit(m.Run())
}

func TestTekton_CreatePipelineRun(t1 *testing.T) {
	type fields struct {
		server    string
		namespace string
		client    *Client
	}
	type args struct {
		ctx context.Context
		pr  *PipelineRun
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantEventID string
		wantErr     bool
	}{
		{
			name: "create pipeline run",
			fields: fields{
				server: "http://" + tektonConfig.Server,
			},
			args: args{
				ctx: context.Background(),
				pr: &PipelineRun{
					Application: "app",
				},
			},
			wantErr:     false,
			wantEventID: "1234",
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &Tekton{
				server:    tt.fields.server,
				namespace: tt.fields.namespace,
				client:    tt.fields.client,
			}
			gotEventID, err := t.CreatePipelineRun(tt.args.ctx, tt.args.pr)
			if (err != nil) != tt.wantErr {
				t1.Errorf("CreatePipelineRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotEventID != tt.wantEventID {
				t1.Errorf("CreatePipelineRun() gotEventID = %v, want %v", gotEventID, tt.wantEventID)
			}
		})
	}
}