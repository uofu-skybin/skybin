SKYBIN_DIR="/Users/tesla/go/src/skybin"
PORTAL_DIR="/Users/tesla/Desktop/skybin-portal"

echo "building linux package"
cd $SKYBIN_DIR
export GOARCH="amd64"
export GOOS="linux"
#rm $PORTAL_DIR/bin/skybin
go build -o $PORTAL_DIR/bin/skybin

cd $PORTAL_DIR
electron-packager . Skybin --icon=assets/cloud-logo_64x64 --overwrite --platform=linux --arch=x64

echo "building OSX package"
cd $SKYBIN_DIR
export GOARCH="amd64"
export GOOS="darwin"
rm $PORTAL_DIR/bin/skybin
go build -o $PORTAL_DIR/bin/skybin

cd $PORTAL_DIR
electron-packager . Skybin --icon=assets/cloud-logo_64x64 --overwrite --platform=darwin --arch=x64

echo "building Windows package"
cd $SKYBIN_DIR
export GOARCH="amd64"
export GOOS="windows"
rm $PORTAL_DIR/bin/skybin
go build -o $PORTAL_DIR/bin/skybin

cd $PORTAL_DIR
electron-packager . Skybin --icon=assets/cloud-logo_64x64 --overwrite --platform=win32 --arch=ia32
