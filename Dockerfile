FROM concourse/buildroot:base

# satisfy go crypto/x509
RUN cat /etc/ssl/certs/*.pem > /etc/ssl/certs/ca-certificates.crt

ADD assets/ /opt/resource/
