cd RPserver
env GOOS=linux go build -o RPserver
chmod +x RPserver
cd ../RPclient
env GOOS=linux go build -o RPclient
chmod +x RPclient
cd ..
