all: build
build:
	$(MAKE) -C cmd/fde_fs
	$(MAKE) -C cmd/personal_fusing

install:
	sudo chown root:root cmd/fde_fs/fde_fs 
	sudo chmod ug+s cmd/fde_fs/fde_fs
	sudo chown root:root cmd/personal_fusing/fde_ptfs
	sudo chmod ug+s cmd/personal_fusing/fde_ptfs
	sudo install cmd/fde_fs/fde_fs /usr/bin/fde_fs -m 755
	sudo install cmd/personal_fusing/fde_ptfs /usr/bin/fde_ptfs -m 755
