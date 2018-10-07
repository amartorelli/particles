# Particles

Particles is a very simple CDN service written in Go.
With Particles you can handle multiple domains by defining them in the configuration file.
It also supports certificates so that you can load the keys and certs of HTTPS website you wish to use the CDN for.

Once started, depending on the configuration, Particles will expose:
- API endpoint: used for administrative tasks
- HTTP endpoint: CDN for HTTP websites
- HTTPS endpoint: CDN for HTTPS websites

For more details look at the configuration section to find out more about how to configure Particles.

## Caching
There are currently two caching solutions:
- memory: stores data into a map kept in memory
- memcached: uses memcached as a backend

## API
An API is exposed on a separte port in order to purge entries from the cache.

## Metrics
Metrics are exposed via Prometheus, using the `/metrics` endpoint of the API server.

## Configuration
The configuration is composed of four sections: api, cache, http, https.
For the api and cache section the configuration is straightforward and specifically to the cache varies depending on which backend is used.
The http and https endpoints are optional and in case backends aren't defined at all for either of them, that particular server won't be started.
For HTTPS backends keys and certs are required in order to load the certificates for the domains.

```
api:
  address: 0.0.0.0
  port : 7546
cache:
  type: "memory"
  options:
    memory_limit: 10240
    ttl: 86400
http:
  port: 80
  backends:
  - name: "example"
    domain: "www.example.com"
    ip: "12.34.56.78"
https:
  port: 443
  backends:
  - name: "secure-example"
    domain: "www.secure-example.com"
    ip: "34.56.78.90"
    key: "/etc/particles/certs/secure-example.key"
    cert: "/etc/particles/certs/secure-example.crt"
```