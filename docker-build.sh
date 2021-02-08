set -euxo pipefail 
docker build "$PWD" -t localhost:32000/nakama-plugin:dkozlov 
