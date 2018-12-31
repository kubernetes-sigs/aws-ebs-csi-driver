# Security Groups (instance firewalls)

# Controller security group (additional security group rules for master nodes in requester account)

# Allow Prometheus to scrape etcd metrics
resource "aws_security_group_rule" "controller-etcd-metrics" {
  depends_on = ["aws_vpc_peering_connection_accepter.peer"]
  provider   = "aws.src"

  security_group_id = "${var.controller_security_group}"

  type      = "ingress"
  protocol  = "tcp"
  from_port = 2381
  to_port   = 2381

  # including peered VPC account ID because it's necessary for using security groups from a peered VPC
  source_security_group_id = "${data.aws_caller_identity.peer.account_id}/${aws_security_group.worker.id}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

resource "aws_security_group_rule" "controller-flannel" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.src"
  security_group_id = "${var.controller_security_group}"

  type                     = "ingress"
  protocol                 = "udp"
  from_port                = 8472
  to_port                  = 8472
  source_security_group_id = "${data.aws_caller_identity.peer.account_id}/${aws_security_group.worker.id}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

# Allow Prometheus to scrape node-exporter daemonset
resource "aws_security_group_rule" "controller-node-exporter" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.src"
  security_group_id = "${var.controller_security_group}"

  type                     = "ingress"
  protocol                 = "tcp"
  from_port                = 9100
  to_port                  = 9100
  source_security_group_id = "${data.aws_caller_identity.peer.account_id}/${aws_security_group.worker.id}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

# Allow apiserver to access kubelets for exec, log, port-forward
resource "aws_security_group_rule" "controller-kubelet" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.src"
  security_group_id = "${var.controller_security_group}"

  type                     = "ingress"
  protocol                 = "tcp"
  from_port                = 10250
  to_port                  = 10250
  source_security_group_id = "${data.aws_caller_identity.peer.account_id}/${aws_security_group.worker.id}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

resource "aws_security_group_rule" "controller-bgp" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.src"
  security_group_id = "${var.controller_security_group}"

  type                     = "ingress"
  protocol                 = "tcp"
  from_port                = 179
  to_port                  = 179
  source_security_group_id = "${data.aws_caller_identity.peer.account_id}/${aws_security_group.worker.id}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

resource "aws_security_group_rule" "controller-ipip" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.src"
  security_group_id = "${var.controller_security_group}"

  type                     = "ingress"
  protocol                 = 4
  from_port                = 0
  to_port                  = 0
  source_security_group_id = "${data.aws_caller_identity.peer.account_id}/${aws_security_group.worker.id}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

resource "aws_security_group_rule" "controller-ipip-legacy" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.src"
  security_group_id = "${var.controller_security_group}"

  type                     = "ingress"
  protocol                 = 94
  from_port                = 0
  to_port                  = 0
  source_security_group_id = "${data.aws_caller_identity.peer.account_id}/${aws_security_group.worker.id}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

# Worker security group

resource "aws_security_group" "worker" {
  provider    = "aws.dst"
  name        = "${var.cluster_name}-worker"
  description = "${var.cluster_name} worker security group"

  vpc_id = "${aws_vpc.network.id}"

  tags = "${map("Name", "${var.cluster_name}-worker")}"
}

resource "aws_security_group_rule" "worker-ssh" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type        = "ingress"
  protocol    = "tcp"
  from_port   = 22
  to_port     = 22
  cidr_blocks = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "worker-http" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type        = "ingress"
  protocol    = "tcp"
  from_port   = 80
  to_port     = 80
  cidr_blocks = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "worker-https" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type        = "ingress"
  protocol    = "tcp"
  from_port   = 443
  to_port     = 443
  cidr_blocks = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "worker-flannel" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type                     = "ingress"
  protocol                 = "udp"
  from_port                = 8472
  to_port                  = 8472
  source_security_group_id = "${data.aws_caller_identity.master.account_id}/${var.controller_security_group}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

resource "aws_security_group_rule" "worker-flannel-self" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type      = "ingress"
  protocol  = "udp"
  from_port = 8472
  to_port   = 8472
  self      = true
}

# Allow Prometheus to scrape node-exporter daemonset
resource "aws_security_group_rule" "worker-node-exporter" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type      = "ingress"
  protocol  = "tcp"
  from_port = 9100
  to_port   = 9100
  self      = true
}

resource "aws_security_group_rule" "ingress-health" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type        = "ingress"
  protocol    = "tcp"
  from_port   = 10254
  to_port     = 10254
  cidr_blocks = ["0.0.0.0/0"]
}

# Allow apiserver to access kubelets for exec, log, port-forward
resource "aws_security_group_rule" "worker-kubelet" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type                     = "ingress"
  protocol                 = "tcp"
  from_port                = 10250
  to_port                  = 10250
  source_security_group_id = "${data.aws_caller_identity.master.account_id}/${var.controller_security_group}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

# Allow Prometheus to scrape kubelet metrics
resource "aws_security_group_rule" "worker-kubelet-self" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type      = "ingress"
  protocol  = "tcp"
  from_port = 10250
  to_port   = 10250
  self      = true
}

resource "aws_security_group_rule" "worker-bgp" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type                     = "ingress"
  protocol                 = "tcp"
  from_port                = 179
  to_port                  = 179
  source_security_group_id = "${data.aws_caller_identity.master.account_id}/${var.controller_security_group}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

resource "aws_security_group_rule" "worker-bgp-self" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type      = "ingress"
  protocol  = "tcp"
  from_port = 179
  to_port   = 179
  self      = true
}

resource "aws_security_group_rule" "worker-ipip" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type                     = "ingress"
  protocol                 = 4
  from_port                = 0
  to_port                  = 0
  source_security_group_id = "${data.aws_caller_identity.master.account_id}/${var.controller_security_group}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

resource "aws_security_group_rule" "worker-ipip-self" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type      = "ingress"
  protocol  = 4
  from_port = 0
  to_port   = 0
  self      = true
}

resource "aws_security_group_rule" "worker-ipip-legacy" {
  depends_on        = ["aws_vpc_peering_connection_accepter.peer"]
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type                     = "ingress"
  protocol                 = 94
  from_port                = 0
  to_port                  = 0
  source_security_group_id = "${data.aws_caller_identity.master.account_id}/${var.controller_security_group}"
  depends_on               = ["aws_vpc_peering_connection_accepter.peer"]
}

resource "aws_security_group_rule" "worker-ipip-legacy-self" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type      = "ingress"
  protocol  = 94
  from_port = 0
  to_port   = 0
  self      = true
}

resource "aws_security_group_rule" "worker-egress" {
  provider          = "aws.dst"
  security_group_id = "${aws_security_group.worker.id}"

  type             = "egress"
  protocol         = "-1"
  from_port        = 0
  to_port          = 0
  cidr_blocks      = ["0.0.0.0/0"]
  ipv6_cidr_blocks = ["::/0"]
}
