# Kubernetes Cluster Telemetry & DaemonSet Deployment

Watchdog gathers cluster-level health indicators, node resource capacities, pod restart frequencies, deployment rollout statuses, and warning events from reachable Kubernetes environments.

---

## ☸️ Kubernetes Collector Features

When connected to a cluster, Watchdog collects:

- **Cluster Context**: Current context name and Kubernetes API server version.
- **Node Statuses**:
  - Node readiness conditions (`Ready` vs `NotReady`).
  - Assigned roles (e.g. `control-plane`, `worker`).
  - CPU and Memory capacity bounds.
  - OS distribution, kernel version, and container runtime version.
- **Pod Health Across Namespaces**:
  - Pod phases (`Running`, `Pending`, `Failed`, `CrashLoopBackOff`).
  - Cumulative restart counters across container statuses.
  - Pod IP addresses and scheduled host node names.
- **Deployment Statuses**:
  - Desired, available, and updated replica tracking.
- **Cluster Event Stream**:
  - Recent cluster warning events, failure reasons, and affected objects.

---

## 🚀 DaemonSet Deployment Manifest

Deploy Watchdog across all nodes in a Kubernetes cluster as a DaemonSet for distributed node observability and Prometheus scraping:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: watchdog-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: watchdog-agent
  namespace: watchdog-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: watchdog-agent-role
rules:
  - apiGroups: [""]
    resources: ["nodes", "pods", "events"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "daemonsets"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: watchdog-agent-binding
subjects:
  - kind: ServiceAccount
    name: watchdog-agent
    namespace: watchdog-system
roleRef:
  kind: ClusterRole
  name: watchdog-agent-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: watchdog-agent
  namespace: watchdog-system
  labels:
    app.kubernetes.io/name: watchdog
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: watchdog
  template:
    metadata:
      labels:
        app.kubernetes.io/name: watchdog
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9100"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: watchdog-agent
      hostPID: true
      hostNetwork: true
      containers:
        - name: watchdog
          image: watchdog:v1.0.0
          imagePullPolicy: IfNotPresent
          command:
            - watchdog
            - server
            - --host=0.0.0.0
            - --port=9100
          ports:
            - name: metrics
              containerPort: 9100
              protocol: TCP
          resources:
            limits:
              cpu: 200m
              memory: 128Mi
            requests:
              cpu: 50m
              memory: 32Mi
          securityContext:
            readOnlyRootFilesystem: true
            runAsNonRoot: true
            runAsUser: 10001
          volumeMounts:
            - name: proc
              mountPath: /host/proc
              readOnly: true
            - name: sys
              mountPath: /host/sys
              readOnly: true
      volumes:
        - name: proc
          hostPath:
            path: /proc
        - name: sys
          hostPath:
            path: /sys
```

---

## 🛠️ Deploying to Kubernetes

```bash
# Apply Watchdog DaemonSet manifest
kubectl apply -f https://raw.githubusercontent.com/DocHoax/watchdog/main/deploy/k8s/daemonset.yaml

# Verify running DaemonSet pods
kubectl get pods -n watchdog-system -o wide
```
