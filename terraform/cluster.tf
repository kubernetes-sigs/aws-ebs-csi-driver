module "aws-riot-radkube" {
  source = "./typhoon/aws/container-linux/kubernetes"

  providers = {
    aws      = "aws.default"
    local    = "local.default"
    null     = "null.default"
    template = "template.default"
    tls      = "tls.default"
  }

  # AWS
  cluster_name = "radkube-canary"
  dns_zone     = "radkube.com"
  dns_zone_id  = "Z2B1JDBLG6KBOS"

  # configuration
  ## This is a PUBLIC key
  ssh_authorized_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCNyIj3pgpE5uI0/eXQwUZJlXlLxF57ovqrIExwdHWjtMYGlOqegvX7bskijxEsYSsy3sxDuiAR6QHiLlzoS0QfeWTaAX+KsAWpL6Xu/STTKa6t7ExSD8xq/6xqmbONRHiwUzfg1F8+nPa7OgAcdf3bTFp3V1iMHLAZBRVGYt9nKgb7s+jId7BxCRYLhTZVj86DZVovNFGaLiyo41KH/FMus33rvcKudNSbGHznL5m8AzdwG0H4OlPu4RRvYwq/Ob65JREnba/Vbdp98oayeQZIVjYRIElZ4b48dk9hy43NqZucCWlY6z5d8ywiFkGiOY5dKySxFncECq76eGfXsHTx"

  asset_dir = ".secrets/clusters/radkube"

  # network config
  ## Using flannel here to support cross-cloud deployments
  ## down the line (looking at you, Azure).
  ## Will be applying the Calico policy addon ("canal")
  networking = "flannel"

  # controller config
  controller_count = 1
  controller_type  = "t2.medium"

  # worker config
  worker_count = 1
  worker_type  = "t2.medium"
}

# Configurations for remote worker pools
module "atcis-cross-account-network" {
  source = "./cross-account-network"

  providers = {
    aws.src = "aws.default"
    aws.dst = "aws.atcis"
  }

  # AWS
  cluster_name = "radkube-canary"

  master_vpc_id  = "${module.aws-riot-radkube.vpc_id}"
  route_table_id = "${module.aws-riot-radkube.route_table_id}"

  master_vpc_cidr = "10.0.0.0/16"
  peer_vpc_cidr   = "10.10.0.0/16"

  controller_security_group = "${module.aws-riot-radkube.controller_security_group}"

  target_group_http  = "${module.atcis-cross-account-workers.target_group_http}"
  target_group_https = "${module.atcis-cross-account-workers.target_group_https}"
}

module "atcis-cross-account-workers" {
  source = "./typhoon/aws/container-linux/kubernetes/workers"

  providers = {
    aws = "aws.atcis"
  }

  # AWS
  vpc_id          = "${module.atcis-cross-account-network.vpc_id}"
  subnet_ids      = "${module.atcis-cross-account-network.subnet_ids}"
  security_groups = "${module.atcis-cross-account-network.worker_security_groups}"

  # configuration
  name               = "radkube-canary"
  kubeconfig         = "${module.aws-riot-radkube.kubeconfig}"
  ssh_authorized_key = "${var.ssh_authorized_key}"

  count         = 1
  instance_type = "t2.medium"
  os_image      = "coreos-stable" # this is the default, but we're setting it explicitly

  # configuration
  ## This is a PUBLIC key
  ssh_authorized_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCNyIj3pgpE5uI0/eXQwUZJlXlLxF57ovqrIExwdHWjtMYGlOqegvX7bskijxEsYSsy3sxDuiAR6QHiLlzoS0QfeWTaAX+KsAWpL6Xu/STTKa6t7ExSD8xq/6xqmbONRHiwUzfg1F8+nPa7OgAcdf3bTFp3V1iMHLAZBRVGYt9nKgb7s+jId7BxCRYLhTZVj86DZVovNFGaLiyo41KH/FMus33rvcKudNSbGHznL5m8AzdwG0H4OlPu4RRvYwq/Ob65JREnba/Vbdp98oayeQZIVjYRIElZ4b48dk9hy43NqZucCWlY6z5d8ywiFkGiOY5dKySxFncECq76eGfXsHTx"
}

module "asdns-cross-account-network" {
  source = "./cross-account-network"

  providers = {
    aws.src = "aws.default"
    aws.dst = "aws.asdns"
  }

  # AWS
  cluster_name = "radkube-canary"

  master_vpc_id   = "${module.aws-riot-radkube.vpc_id}"
  route_table_id  = "${module.aws-riot-radkube.route_table_id}"
  master_vpc_cidr = "10.0.0.0/16"
  peer_vpc_cidr   = "10.11.0.0/16"

  controller_security_group = "${module.aws-riot-radkube.controller_security_group}"

  target_group_http  = "${module.asdns-cross-account-workers.target_group_http}"
  target_group_https = "${module.asdns-cross-account-workers.target_group_https}"
}

module "asdns-cross-account-workers" {
  # Workers for ASDNS (bx) AWS account
  source = "./typhoon/aws/container-linux/kubernetes/workers"

  providers = {
    aws = "aws.asdns"
  }

  # AWS
  vpc_id          = "${module.asdns-cross-account-network.vpc_id}"
  subnet_ids      = "${module.asdns-cross-account-network.subnet_ids}"
  security_groups = "${module.asdns-cross-account-network.worker_security_groups}"

  # configuration
  name               = "radkube-canary"
  kubeconfig         = "${module.aws-riot-radkube.kubeconfig}"
  ssh_authorized_key = "${var.ssh_authorized_key}"

  count         = 1
  instance_type = "t2.medium"
  os_image      = "coreos-stable" # this is the default, but we're setting it explicitly

  # configuration
  ## This is a PUBLIC key
  ssh_authorized_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCNyIj3pgpE5uI0/eXQwUZJlXlLxF57ovqrIExwdHWjtMYGlOqegvX7bskijxEsYSsy3sxDuiAR6QHiLlzoS0QfeWTaAX+KsAWpL6Xu/STTKa6t7ExSD8xq/6xqmbONRHiwUzfg1F8+nPa7OgAcdf3bTFp3V1iMHLAZBRVGYt9nKgb7s+jId7BxCRYLhTZVj86DZVovNFGaLiyo41KH/FMus33rvcKudNSbGHznL5m8AzdwG0H4OlPu4RRvYwq/Ob65JREnba/Vbdp98oayeQZIVjYRIElZ4b48dk9hy43NqZucCWlY6z5d8ywiFkGiOY5dKySxFncECq76eGfXsHTx"
}
