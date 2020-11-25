
## 快速使用

### 部署自定义资源

```shell script
kubectl apply -f ./artifact/crd.yaml
```

### 部署一个资源对象

```shell script
kubectl apply -f ./artifact/stu.yaml
```

### 构建controller

```shell script
./script/gene-codes.sh

go build ./cmd/stu-controller/
```

### 运行controller

```shell script
./stu-controller -kubeconfig ~/.kube/config
```
