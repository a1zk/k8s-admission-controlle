#!/bin/bash
#Buid script for creating docker image and generate Admission controler.
set -o errexit
set -o nounset
set -o pipefail

read -p "Write DockerHub repo" tag


echo " ==================================== "
echo -e "\e[32mCreate Docker Image\e[0m"
echo " ==================================== "
docker build --no-cache -t $tag/k8s-ac:1.0.0 .

echo " ==================================== "
echo -e "\e[32mCreate Secret for AC\e[0m"
echo " ==================================== "
bash scripts/webhook-create-signed-cert.sh  --service k8s-ac-svc  --secret k8s-ac

echo " ==================================== "
echo -e "\e[32mDeploy AC to K8S\e[0m"
echo " ==================================== "
export CA_BUNDLE=$(kubectl config view --raw -o json|jq -r '.clusters[0].cluster."certificate-authority"'|xargs cat|base64|tr -d '\n')
cat k8s-valid-temp.yaml|envsubst>validation.yaml
kubect apply -f validation.yaml
kubect apply -f k8s-deployment.yaml
kubect apply -f k8s-svc.yaml
