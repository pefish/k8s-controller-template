#!/usr/bin/env bash

go mod tidy

go mod vendor

chmod +x ./vendor/k8s.io/code-generator/generate-groups.sh

./vendor/k8s.io/code-generator/generate-groups.sh "deepcopy,client,informer,lister" github.com/pefish/k8s-controller-template/pkg/generated github.com/pefish/k8s-controller-template/pkg/apis pefish:v1alpha1 --go-header-file ./hack/boilerplate.go.txt --output-base ./

cp -fRap github.com/pefish/* ../ && rm -rf github.com

rm -rf vendor

echo "done!!!"
