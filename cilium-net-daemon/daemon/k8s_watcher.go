package daemon

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/noironetworks/cilium-net/common/types"

	"k8s.io/kubernetes/pkg/watch"
)

type networkPolicyWatchEvent struct {
	Type   watch.EventType     `json:"type"`
	Object types.NetworkPolicy `json:"object"`
}

func (d *Daemon) EnableK8sWatcher(maxSeconds time.Duration) error {
	curSeconds := time.Second

	u := d.k8sClient.Get().RequestURI("apis/experimental.kubernetes.io/v1").
		Namespace("default").Resource("networkpolicys").Param("watch", "true").URL()
	go func() {
		makeRequest := func() *http.Response {
			for {
				resp, err := http.Get(u.String())
				if err != nil {
					log.Errorf("Error while getting URL %s: %s", u, err)
				} else if resp.StatusCode == http.StatusOK {
					return resp
				} else {
					log.Debugf("Unable to kubernetes network policies, please insert some policies")
				}
				time.Sleep(curSeconds)
				if curSeconds < maxSeconds {
					curSeconds += time.Second
				}
			}
		}
		resp := makeRequest()
		curSeconds = time.Second
		log.Info("Listening for kubernetes network policys changes")
		for {
			npwe := networkPolicyWatchEvent{}
			err := json.NewDecoder(resp.Body).Decode(&npwe)
			if err != nil {
				log.Errorf("Error while receiving data %s", err)
				resp.Body.Close()
				resp = makeRequest()
				curSeconds = time.Second
				continue
			}
			log.Debugf("Received kubernetes network policy %+v\n", npwe)
			go d.processNPE(npwe)
		}
	}()
	return nil
}

func (d *Daemon) processNPE(npwe networkPolicyWatchEvent) {
	nodePath, pn, err := types.K8sNP2CP(npwe.Object)
	if err != nil {
		log.Errorf("Error while parsing kubernetes network policy %+v: %s", npwe.Object, err)
		return
	}
	switch npwe.Type {
	case watch.Added, watch.Modified:
		if err := d.PolicyAdd(nodePath, pn); err != nil {
			log.Errorf("Error while adding kubernetes network policy %+v: %s", pn, err)
			return
		}
		log.Info("Kubernetes network policy successfully add")
	case watch.Deleted:
		if err := d.PolicyDelete(nodePath); err != nil {
			log.Errorf("Error while deleting kubernetes network policy %+v: %s", pn, err)
			return
		}
		log.Info("Kubernetes network policy successfully removed")
	}
}
