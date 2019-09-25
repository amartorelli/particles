# Particles

[![Build Status](https://travis-ci.org/amartorelli/particles.svg?branch=master)](https://travis-ci.org/amartorelli/particles)

Particles is a very simple CDN service written in Go.
With Particles you can handle multiple domains by defining them in the configuration file.
It also supports certificates so that you can load the keys and certs of HTTPS website you wish to use the CDN for.

Once started, depending on the configuration, Particles will expose:

* API endpoint: used for administrative tasks
* HTTP endpoint: CDN for HTTP websites
* HTTPS endpoint: CDN for HTTPS websites

For more details look at the configuration section to find out more about how to configure Particles.

## Caching

There are currently two caching solutions:

* memory: stores data into a map kept in memory
* memcached: uses memcached as a backend

## API

An API is exposed on a separte port in order to purge entries from the cache.
To purge a cache entry:

```bash
curl http://localhost:7546/purge -d '{"resource": "http://www.example.com:80/wp-content/uploads/2017/03/banner.jpg"}'
```

## Metrics

Metrics are exposed via Prometheus, using the `/metrics` endpoint of the API server:

```bash
curl http://localhost:7546/metrics
```

## Configuration

The configuration is composed of four sections: api, cache, http, https.
For the api and cache section the configuration is straightforward and specifically to the cache varies depending on which backend is used.
The http and https endpoints are optional and in case backends aren't defined at all for either of them, that particular server won't be started.
For HTTPS backends keys and certs are required in order to load the certificates for the domains.

```yaml
api:
  address: 0.0.0.0
  port : 7546
cache:
  type: "memory"
  options:
    memory_limit: 10240
    ttl: 86400
    patterns: "test/css"
http:
  port: 80
  backends:
    - name: "example"
      domain: "www.example.com"
      ip: "12.34.56.78"
      port: 8080
      ifmodified_validation: 450
https:
  port: 443
  backends:
    - name: "secure-example"
      domain: "www.secure-example.com"
      ip: "34.56.78.90"
      port: 8443
```

### Configuration parameters

| Parameter | Description | Default | Required |
|---|---|---|---|
| api.address | The listening address of the API  | `0.0.0.0` | no |
| api.port | The port the API listens on | `7546` | no |
| api.cert | The certificate to use for the API endpoints | `""` | no |
| api.key | The key for the certificate to use for the API endpoints | `""` | no |
| cache.type | The type of cache to use | `"memory"` | no |
| cache.options | Dynamic map varying based on the cache type | `{memory_limit": "10240", "ttl": "86400"}` | no |
| http.address | The listening address to receive HTTP traffic | `"0.0.0.0"` | no |
| http.port | The port to receive HTTP traffic | `80` | no |
| http.backends | List of backends handled on the HTTP port | `[]` | no |
| https.address | The listening address to receive HTTPS traffic  | `"0.0.0.0"` | no |
| https.port | The port to receive HTTPS traffic | `443` | no |
| https.backends | List of backends handled on the HTTPS port | `[]` | no |

### Cache options configuration

#### Memory

| Parameter | Description | Default | Required |
|---|---|---|---|
| memory_limit | The memory size allocatable | `1073741824` | no |
| patterns | The content-types to cache expressed as regexp | `"^(image|audio|video)/.+$|^.+/javascript.*$|^text/css$"` | no |
| force_purge | Delete random items if memory can't be freed up | `true` | no |

#### Memcached

| Parameter | Description | Default | Required |
|---|---|---|---|
| endpoints | Comma separated list of memcached endpoints | `"127.0.0.1:11211"` | no |
| patterns | The content-types to cache expressed as regexp | `"^(image|audio|video)/.+$|^.+/javascript.*$|^text/css$"` | no |

### Backend configuration

| Parameter | Description | Default | Required |
|---|---|---|---|
| name | The name of the backend | `-` | yes |
| domain | The domain for the backend | `-` | yes |
| ip | The IP of the original source for this backend | `-` | yes |
| port | The port of the original source for this backend | `80` if defined in the HTTP section or `443` if in the HTTPS | no |
| ifmodified_validation | The amount of seconds to wait before issuing an `If-Not-Modified` request| `300` | no |

*Note* that for each backend you can optionally specify an IP. This will cause the HTTP client to override the DNS
results and point to that specific IP address.
The same configuration is valid for the port: backends listening on different ports than the one used by Particles are supported, just override the port
by defining a new one for each backend you wish to override.
Again, IP and port for backends are absolutely optional and Particles would use by default the same port defined for the HTTP
or HTTPS Particles endpoints.
