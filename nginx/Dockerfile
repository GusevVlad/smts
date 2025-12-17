FROM nginx:alpine

# Remove default configuration
RUN rm /etc/nginx/conf.d/default.conf

# Copy custom configuration
COPY ./ext/nginx.conf /etc/nginx/nginx.conf

# Expose ports
EXPOSE 80 443

CMD ["nginx", "-g", "daemon off;"]