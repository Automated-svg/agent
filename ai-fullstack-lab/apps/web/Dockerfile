FROM node:24-bookworm-slim AS build
WORKDIR /app
COPY package.json package-lock.json ./
COPY apps/web/package.json ./apps/web/package.json
RUN npm ci
COPY apps/web ./apps/web
ARG API_ORIGIN=http://api:8080
ENV API_ORIGIN=$API_ORIGIN NEXT_TELEMETRY_DISABLED=1
RUN npm run build -w apps/web && chown -R node:node /app/apps/web/.next
ENV NODE_ENV=production
USER node
EXPOSE 3000
CMD ["npm", "run", "start", "-w", "apps/web", "--", "--hostname", "0.0.0.0"]
