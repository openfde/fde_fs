ver=`git log --pretty=format:"%h" -1`
tag=`git describe --abbrev=0 --tags`
date1=`date +%F_%T`
build:
	go build -ldflags "-X main._version_=$(ver) -X main._tag_=$(tag) -X main._date_=$(date1)"
install:
	sudo cp -a fde_fs /usr/bin/
	sudo chown root:root /usr/bin/fde_fs
	sudo chmod u+s /usr/bin/fde_fs
