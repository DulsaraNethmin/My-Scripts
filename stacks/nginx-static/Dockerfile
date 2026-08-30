FROM nginx:1.27-alpine

# SPA-aware config: unknown paths fall back to index.html.
COPY nginx.conf /etc/nginx/conf.d/default.conf

# A placeholder page so a fresh `spinup up nginx-static` shows something.
# Anything you put in the site volume replaces it.
COPY site/ /usr/share/nginx/html/

EXPOSE 80
