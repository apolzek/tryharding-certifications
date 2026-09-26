"""Um site HTTPS mínimo (certificado assinado pela CA do lab) para o blackbox medir TLS."""
import http.server
import ssl

srv = http.server.ThreadingHTTPServer(("", 8443), http.server.SimpleHTTPRequestHandler)
ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain("/certs/site.crt", "/certs/site.key")
srv.socket = ctx.wrap_socket(srv.socket, server_side=True)
print("https-site ouvindo em :8443")
srv.serve_forever()
