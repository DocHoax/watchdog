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

## 🚀 Production Deployment Architecture

Watchdog is deployed to Kubernetes clusters as a hardened DaemonSet across all nodes. The deployment is organized into modular manifests under `deploy/k8s/`:

- `deploy/k8s/daemonset.yaml`: Core DaemonSet with Namespace, ServiceAccount, ClusterRole, ClusterRoleBinding, host mounts, and security hardening.
- `deploy/k8s/configmap.yaml`: Centralized configuration template.
- `deploy/k8s/secret.yaml`: Authentication Secret template for Bearer token validation.
- `deploy/k8s/service.yaml`: ClusterIP and Headless Service definitions exposing port 9100.
- `deploy/k8s/servicemonitor.yaml`: Prometheus Operator ServiceMonitor for automated metric scraping.

### 1. Manifest Deployment
```bash
# Apply ConfigMap, Secret, DaemonSet, Service, and ServiceMonitor
kubectl apply -f deploy/k8s/configmap.yaml
kubectl apply -f deploy/k8s/secret.yaml
kubectl apply -f deploy/k8s/daemonset.yaml
kubectl apply -f deploy/k8s/service.yaml
kubectl apply -f deploy/k8s/servicemonitor.yaml
```

### 2. Hardened DaemonSet Specification (`deploy/k8s/daemonset.yaml`)
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: watchdog
  namespace: watchdog-system
  labels:
    app.kubernetes.io/name: watchdog
    app.kubernetes.io/part-of: watchdog
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
      hostNetwork: true
      hostPID: true
      containers:
        - name: watchdog
          image: watchdog:latest
          imagePullPolicy: IfNotPresent
          args:
            - "server"
            - "--port"
            - "9100"
            - "--config"
            - "/etc/watchdog/config.yaml"
            - "--token-file"
            - "/etc/watchdog/secrets/token"
          ports:
            - name: prometheus
              containerPort: 9100
              hostPort: 9100
          livenessProbe:
            httpGet:
              path: /healthz
              port: 9100
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readyz
              port: 9100
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3
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
            runAsUser: 1000
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
          volumeMounts:
            - name: proc
              mountPath: /host/proc
              readOnly: true
            - name: sys
              mountPath: /host/sys
              readOnly: true
            - name: var-run-docker
              mountPath: /var/run/docker.sock
              readOnly: true
            - name: watchdog-config
              mountPath: /etc/watchdog/config.yaml
              subPath: config.yaml
              readOnly: true
            - name: watchdog-storage
              mountPath: /var/lib/watchdog
            - name: watchdog-auth-secret
              mountPath: /etc/watchdog/secrets
              readOnly: true
      volumes:
        - name: proc
          hostPath:
            path: /proc
        - name: sys
          hostPath:
            path: /sys
        - name: var-run-docker
          hostPath:
            path: /var/run/docker.sock
        - name: watchdog-config
          configMap:
            name: watchdog-config
        - name: watchdog-storage
          emptyDir: {}
        - name: watchdog-auth-secret
          secret:
            secretName: watchdog-auth
            optional: true
```

---

## 🔒 Security Posture & Least Privilege

1. **Pod Security Standards**: Operates with `runAsNonRoot: true`, `runAsUser: 1000`, `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, and drops all Linux capabilities (`drop: ["ALL"]`).
2. **Read-Only Host Mounts**: `/proc`, `/sys`, and `/var/run/docker.sock` are mounted with `readOnly: true` ensuring Watchdog cannot alter host state.
3. **Secret Rotation**: Tokens are mounted as read-only volume files (`/etc/watchdog/secrets/token`). Rotating the Kubernetes Secret updates the file on disk without requiring pod recreation when token reload is triggered.
4. **RBAC Least Privilege**: The `watchdog-agent` ClusterRole is restricted to `get`, `list`, and `watch` verbs on read-only cluster resources (`nodes`, `services`, `endpoints`, `pods`, `namespaces`, `daemonsets`, `statefulsets`, `deployments`).

---

## 🩺 Health & Readiness Probe Triage

- **Liveness Failures (`/healthz`)**: Indicates that the process or HTTP event loop is deadlocked or unresponsive. Kubernetes will automatically restart the container.
- **Readiness Failures (`/readyz`)**: Indicates that the collector manager has not yet populated an initial telemetry snapshot or storage is unreachable. The pod is removed from endpoints until ready, preventing failed scrape requests.
