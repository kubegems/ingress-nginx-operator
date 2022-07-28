# Ingress Nginx Operator

Ingress Nginx Operator is the operator for [Kubernetes Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx). We use this project to manage nginx gateway resources in [Kubegems](https://github.com/kubegems/kubegems).

This project is based on [nginx-ingress-operator by Nginx Inc](https://github.com/nginxinc/nginx-ingress-operator) and the main code structure is learn from it. The main difference is:

[Nginx-ingress-operator by Nginx Inc](https://github.com/nginxinc/nginx-ingress-operator) uses https://github.com/nginxinc/kubernetes-ingress controller. **This project uses https://github.com/kubernetes/ingress-nginx controller**

The following table shows the relation between ingress-nginx operator and controller project.

| Ingress NGINX Operator | Ingress NGINX Controller |
| ---------------------- | ------------------------ |
| 1.0.0                  | 1.3.0                    |

## Getting Started

### Install ingress-nginx-operator

1. Deploy
```bash
kubectl apply -f https://raw.githubusercontent.com/kubegems/ingress-nginx-operator/main/bundle.yaml
```

2. Check
```bash
kubectl  get pod -n ingress-nginx-operator-system
NAME                                                         READY   STATUS    RESTARTS   AGE
ingress-nginx-operator-controller-manager-865d785459-psjgr   2/2     Running   0          1m
```

### Deploy ingress-nginx controller 
This is an example:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubegems/ingress-nginx-operator/main/config/samples/networking_v1beta1_nginxingresscontroller.yaml
```

## Development

### Run local
1. Deploy
```bash
make manifest
make install
```

2. Run
```bash
make generate
make run
```
