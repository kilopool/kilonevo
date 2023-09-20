mkdir -p ./randomx/RandomX/build
cd ./randomx/RandomX/build/
cmake ..
make
cd ../rxlib
./make.sh
cd ../../../
go mod tidy
go build .
