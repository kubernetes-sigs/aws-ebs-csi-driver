data "aws_availability_zones" "all" {}

# Network VPC, gateway, and routes

resource "aws_vpc" "network" {
  provider = "aws.dst"
  cidr_block                       = "${var.peer_vpc_cidr}"
  assign_generated_ipv6_cidr_block = true
  enable_dns_support               = true
  enable_dns_hostnames             = true

  tags = "${map("Name", "${var.cluster_name}")}"
}

resource "aws_internet_gateway" "gateway" {
  provider = "aws.dst"
  vpc_id = "${aws_vpc.network.id}"

  tags = "${map("Name", "${var.cluster_name}")}"
}

resource "aws_route_table" "default" {
  provider = "aws.dst"

  vpc_id = "${aws_vpc.network.id}"

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = "${aws_internet_gateway.gateway.id}"
  }

  route {
    ipv6_cidr_block = "::/0"
    gateway_id      = "${aws_internet_gateway.gateway.id}"
  }

  route {
    cidr_block = "${var.master_vpc_cidr}"
    vpc_peering_connection_id = "${aws_vpc_peering_connection.peer.id}"
  }
  tags = "${map("Name", "${var.cluster_name}")}"
}

resource "aws_route" "peer_ipv4" {
  provider = "aws.src"
  route_table_id            = "${var.route_table_id}"
  destination_cidr_block    = "${aws_vpc.network.cidr_block}"
  vpc_peering_connection_id = "${aws_vpc_peering_connection.peer.id}"
}

resource "aws_route" "peer_ipv6" {
  provider = "aws.src"
  route_table_id            = "${var.route_table_id}"
  destination_cidr_block    = "${aws_vpc.network.ipv6_cidr_block}"
  vpc_peering_connection_id = "${aws_vpc_peering_connection.peer.id}"
}
# Subnets (one per availability zone)

resource "aws_subnet" "public" {
  provider = "aws.dst"

  count = "${length(data.aws_availability_zones.all.names)}"

  vpc_id            = "${aws_vpc.network.id}"
  availability_zone = "${data.aws_availability_zones.all.names[count.index]}"

  cidr_block                      = "${cidrsubnet(aws_vpc.network.cidr_block, 4, count.index)}"
  ipv6_cidr_block                 = "${cidrsubnet(aws_vpc.network.ipv6_cidr_block, 8, count.index)}"
  map_public_ip_on_launch         = true
  assign_ipv6_address_on_creation = true

  tags = "${map("Name", "${var.cluster_name}-public-${count.index}")}"
}

resource "aws_route_table_association" "public" {
  provider = "aws.dst"

  count = "${length(data.aws_availability_zones.all.names)}"

  route_table_id = "${aws_route_table.default.id}"
  subnet_id      = "${element(aws_subnet.public.*.id, count.index)}"
}
