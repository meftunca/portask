#!/bin/bash

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   🚀 Portask Full System Startup 🚀   ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}❌ Docker is not running. Please start Docker first.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Docker is running${NC}"
echo ""

# Generate JWT secret if not exists
if [ ! -f .env ]; then
    echo -e "${YELLOW}📝 Creating .env file...${NC}"
    JWT_SECRET=$(openssl rand -base64 32)
    cat > .env << EOF
JWT_SECRET=${JWT_SECRET}
GRAFANA_PASSWORD=admin
EOF
    echo -e "${GREEN}✅ .env file created${NC}"
else
    echo -e "${GREEN}✅ .env file exists${NC}"
fi

# Stop existing containers
echo -e "${YELLOW}🛑 Stopping existing containers...${NC}"
docker-compose -f docker-compose.full.yml down 2>/dev/null || true

# Build images
echo -e "${YELLOW}🏗️  Building Portask image...${NC}"
docker-compose -f docker-compose.full.yml build

# Start services
echo -e "${GREEN}🚀 Starting all services...${NC}"
docker-compose -f docker-compose.full.yml up -d

# Wait for services to be healthy
echo -e "${YELLOW}⏳ Waiting for services to be healthy...${NC}"
sleep 5

# Check service health
echo ""
echo -e "${BLUE}📊 Service Status:${NC}"
echo ""

check_service() {
    local service_name=$1
    local url=$2
    local max_retries=30
    local retry=0

    while [ $retry -lt $max_retries ]; do
        if curl -s -f "$url" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ $service_name is healthy${NC}"
            return 0
        fi
        retry=$((retry + 1))
        sleep 1
    done

    echo -e "${RED}❌ $service_name failed to start${NC}"
    return 1
}

# Check each service
check_service "Dragonfly" "http://localhost:6379" || true
check_service "Portask" "http://localhost:8080/health"
check_service "Prometheus" "http://localhost:9091/-/healthy"
check_service "Grafana" "http://localhost:3000/api/health"

# Print access information
echo ""
echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        🎉 System is Ready! 🎉         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}📍 Service URLs:${NC}"
echo ""
echo -e "  🚀 Portask API:       ${BLUE}http://localhost:8080${NC}"
echo -e "  📊 Metrics:           ${BLUE}http://localhost:8080/metrics${NC}"
echo -e "  💚 Health Check:      ${BLUE}http://localhost:8080/health${NC}"
echo -e "  📈 Prometheus:        ${BLUE}http://localhost:9091${NC}"
echo -e "  📊 Grafana:           ${BLUE}http://localhost:3000${NC} (admin/admin)"
echo -e "  🗄️  Dragonfly:         ${BLUE}localhost:6379${NC}"
echo ""
echo -e "${GREEN}📚 Quick Commands:${NC}"
echo ""
echo -e "  View logs:            ${YELLOW}docker-compose -f docker-compose.full.yml logs -f${NC}"
echo -e "  Stop system:          ${YELLOW}docker-compose -f docker-compose.full.yml down${NC}"
echo -e "  Restart system:       ${YELLOW}docker-compose -f docker-compose.full.yml restart${NC}"
echo -e "  Check status:         ${YELLOW}docker-compose -f docker-compose.full.yml ps${NC}"
echo ""
echo -e "${GREEN}🧪 Test the system:${NC}"
echo ""
echo -e "  Health Check:         ${YELLOW}curl http://localhost:8080/health${NC}"
echo -e "  Metrics:              ${YELLOW}curl http://localhost:8080/metrics${NC}"
echo ""
echo -e "${GREEN}🎯 Next Steps:${NC}"
echo ""
echo -e "  1. Open Grafana at http://localhost:3000"
echo -e "  2. Login with admin/admin"
echo -e "  3. Explore Portask dashboard"
echo -e "  4. Check Prometheus at http://localhost:9091"
echo -e "  5. Test API endpoints"
echo ""
echo -e "${BLUE}Happy messaging with Portask! 🚀${NC}"
echo ""

