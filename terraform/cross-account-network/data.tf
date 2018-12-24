data "aws_caller_identity" "master" {
  provider = "aws.dst"
}

data "aws_caller_identity" "peer" {
  provider = "aws.dst"
}
