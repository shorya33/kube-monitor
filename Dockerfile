# Stage 1: Build the Go app
FROM golang:1.21 AS backend

WORKDIR /app

COPY . .
COPY go.mod ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o main .
# Stage 2: Create a distroless image
FROM scratch AS backend-distroless

COPY --from=backend /app/main /

# Expose the port your app listens on (default is 8080 for Go apps)
EXPOSE 8080

CMD ["/main"]

FROM nginx:alpine AS frontend

# Copy the frontend files into the Nginx web directory
COPY index.html /usr/share/nginx/html/index.html

# Expose port 80 to serve the website
EXPOSE 80