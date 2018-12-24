variable "cluster_name" {
  description = "Name of the master cluster"
  type        = "string"
}

variable "master_vpc_id" {
  description = "ID of master VPC"
  type        = "string"
}

variable "master_vpc_cidr" {
  description = "CIDR IPv4 range to assign to EC2 nodes"
  type        = "string"
  default     = "10.0.0.0/16"
}

variable "peer_vpc_cidr" {
  description = "CIDR IPv4 range to assign to peered VPC EC2 nodes"
  type        = "string"
  default     = "10.10.0.0/16"
}

variable "controller_security_group" {
  description = "ID of security group for main account master"
  type        = "string"
}