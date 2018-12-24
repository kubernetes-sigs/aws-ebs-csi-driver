module "aws-riot-radkube" {
  source = "git::https://github.com/poseidon/typhoon//aws/container-linux/kubernetes?ref=v1.13.0"

  providers = {
    aws = "aws.default"
    local = "local.default"
    null = "null.default"
    template = "template.default"
    tls = "tls.default"
  }

  # AWS
  cluster_name = "radkube"
  dns_zone     = "radkube.com"
  dns_zone_id  = "Z2B1JDBLG6KBOS"

  # configuration
  ## This is a PUBLIC key
  ssh_authorized_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC1VM2bo/ONL722tB/x5N6q0CgGT9UO9AG7Srw1AUFld6gz/Vh6x68Efj+hqLOUOJhK/ouXLeBoX06jGFDwJa/nvAsM+u/Z5rXD+niMCMRrq3rKJ9rDj37EnVhXa36F0e79j57L5HBIt6PVdSvs2rUPSPLAMExI3CMF0R9G0Gl2uJbxosRf41gJ4NY/D32jYrReEuAU3JT6FDWpLwSeEK4a9eUpKnrUFLtaybqpRhV2rSRtCoMlFcHBlF6WElNI5ahGnKIEXgrU97npZMWOiEheiyp7fYjpiIjcWn3Z/kKJQK3Js3tmwLdUvdRWexF3Ezo2rZhGqQmHpNzBKBpcaSfb"
  asset_dir          = "/home/shanesiebken/.secrets/clusters/radkube"


  # network config
  ## Using flannel here to support cross-cloud deployments
  ## down the line (looking at you, Azure).
  ## Will be applying the Calico policy addon ("canal")
  networking = "flannel"

  # controller config
  controller_count = 1
  controller_type = "t2.medium"

  # worker config
  worker_count = 1
  worker_type  = "t2.medium"
}

module "utap-cross-account-network" {
  source = "cross-account-network"
  
  providers = {
    aws.src = "aws.default"
    aws.dst = "aws.utap"
  }

  # AWS
  cluster_name = "radkube"

  master_vpc_id = "${module.aws-riot-radkube.vpc_id}"

  master_vpc_cidr = "10.0.0.0/16"
  peer_vpc_cidr = "10.10.0.0/16"

  controller_security_group =  "sg-0eda6aa38b13c4c12"
}

module "utap-cross-account-workers" {
  source = "git::https://github.com/poseidon/typhoon//aws/container-linux/kubernetes/workers?ref=v1.13.1"

  providers = {
    aws = "aws.utap"
  }

  # AWS
  vpc_id          = "${module.utap-cross-account-network.vpc_id}"
  subnet_ids      = "${module.utap-cross-account-network.subnet_ids}"
  security_groups = "${module.utap-cross-account-network.worker_security_groups}"

  # configuration
  name               = "radkube-workers"
  kubeconfig         = "${module.aws-riot-radkube.kubeconfig}"
  ssh_authorized_key = "${var.ssh_authorized_key}"

  count         = 1
  instance_type = "t2.medium"
  os_image      = "coreos-stable" # this is the default, but we're setting it explicitly

    # configuration
  ## This is a PUBLIC key
  ssh_authorized_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCNyIj3pgpE5uI0/eXQwUZJlXlLxF57ovqrIExwdHWjtMYGlOqegvX7bskijxEsYSsy3sxDuiAR6QHiLlzoS0QfeWTaAX+KsAWpL6Xu/STTKa6t7ExSD8xq/6xqmbONRHiwUzfg1F8+nPa7OgAcdf3bTFp3V1iMHLAZBRVGYt9nKgb7s+jId7BxCRYLhTZVj86DZVovNFGaLiyo41KH/FMus33rvcKudNSbGHznL5m8AzdwG0H4OlPu4RRvYwq/Ob65JREnba/Vbdp98oayeQZIVjYRIElZ4b48dk9hy43NqZucCWlY6z5d8ywiFkGiOY5dKySxFncECq76eGfXsHTx"
}
