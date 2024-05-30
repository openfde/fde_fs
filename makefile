all: build
build:
	$(MAKE) -C cmd/fde_fs
	$(MAKE) -C cmd/personal_fusing

install:
	sudo chown root:root cmd/fde_fs/fde_fs 
	sudo chmod ug+s cmd/fde_fs/fde_fs
	sudo chown root:root cmd/personal_fusing/fde_ptfs
	sudo chmod ug+s cmd/personal_fusing/fde_ptfs
	sudo cp -a cmd/fde_fs/fde_fs /usr/bin/fde_fs 
	sudo cp -a cmd/personal_fusing/fde_ptfs /usr/bin/fde_ptfs
