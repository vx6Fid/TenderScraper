#!/bin/sh
# Runs automatically inside the LocalStack container once it is ready.
# Creates the S3 bucket the tender-scraper expects. Idempotent.
awslocal s3 mb "s3://${S3_BUCKET:-tenderbharat-ap-south-1}" 2>/dev/null || true
echo "[localstack-init] ensured bucket ${S3_BUCKET:-tenderbharat-ap-south-1} exists"
