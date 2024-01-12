cd cmd/GoExposeServer
env GOOS=linux go build -o main
chmod +x RPserver
cd ../GoExposeClient
env GOOS=linux go build -o main
chmod +x RPclient
cd ../..
