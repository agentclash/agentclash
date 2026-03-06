#!/bin/sh

# Deployment script for ACME API stack

echo "Starting deployment..."

# Pull latest images
docker-compose pull

# Build app image
docker-compose build app

# Run database migrations
docker-compose run --rm app npm run migrate

# Start all services
docker-compose up -d

# Wait for services
sleep 10

# Health check
curl -f http://localhost:80/health || echo "Health check failed!"

echo "Deployment complete."
