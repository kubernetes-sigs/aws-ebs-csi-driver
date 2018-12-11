module "aws-tempest" {
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
  ssh_authorized_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCNyIj3pgpE5uI0/eXQwUZJlXlLxF57ovqrIExwdHWjtMYGlOqegvX7bskijxEsYSsy3sxDuiAR6QHiLlzoS0QfeWTaAX+KsAWpL6Xu/STTKa6t7ExSD8xq/6xqmbONRHiwUzfg1F8+nPa7OgAcdf3bTFp3V1iMHLAZBRVGYt9nKgb7s+jId7BxCRYLhTZVj86DZVovNFGaLiyo41KH/FMus33rvcKudNSbGHznL5m8AzdwG0H4OlPu4RRvYwq/Ob65JREnba/Vbdp98oayeQZIVjYRIElZ4b48dk9hy43NqZucCWlY6z5d8ywiFkGiOY5dKySxFncECq76eGfXsHTx"
  asset_dir          = "/home/shane/.secrets/clusters/tempest"


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