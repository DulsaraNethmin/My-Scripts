# Use an official Nginx image as the base
FROM nginx:alpine

# Set working directory to /usr/share/nginx/html (default directory for Nginx)
WORKDIR /usr/share/nginx/html

# Remove the default Nginx static files
RUN rm -rf ./*

# Copy the built static files from the local dist directory into the container
# COPY ./dist /usr/share/nginx/html

# Copy a custom Nginx configuration file
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose port 80 to access the application
EXPOSE 80

# Start Nginx when the container runs
CMD ["nginx", "-g", "daemon off;"]
