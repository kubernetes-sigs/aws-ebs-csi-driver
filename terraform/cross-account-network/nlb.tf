# Network Load Balancer for apiservers and ingress
resource "aws_lb" "nlb" {
  provider = "aws.dst"
  name               = "${var.cluster_name}-nlb"
  load_balancer_type = "network"
  internal           = false

  subnets = ["${aws_subnet.public.*.id}"]

  enable_cross_zone_load_balancing = true
}

# Forward HTTP ingress traffic to workers
resource "aws_lb_listener" "ingress-http" {
  provider = "aws.dst"
  load_balancer_arn = "${aws_lb.nlb.arn}"
  protocol          = "TCP"
  port              = 80

  default_action {
    type             = "forward"
    target_group_arn = "${var.target_group_http}"
  }
}

# Forward HTTPS ingress traffic to workers
resource "aws_lb_listener" "ingress-https" {
  provider = "aws.dst"
  load_balancer_arn = "${aws_lb.nlb.arn}"
  protocol          = "TCP"
  port              = 443

  default_action {
    type             = "forward"
    target_group_arn = "${var.target_group_https}"
  }
}
