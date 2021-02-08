set -euxo pipefail 
bash docker-build.sh 
pushd ../nakama
bash docker-build.sh
kubectl delete -f  kubernetes/04-nakama-deployment.yml
kubectl apply -f  kubernetes/04-nakama-deployment.yml
popd
