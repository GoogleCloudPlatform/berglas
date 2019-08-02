import os

from flask import Flask, Response
import berglas.auto  # noqa

app = Flask(__name__)

@app.route('/')
def f():
    api_key = os.environ.get('API_KEY')
    tls_key_path = os.environ.get('TLS_KEY')

    tls_key = "file does not exist"
    if tls_key_path and os.path.isfile(tls_key_path):
        tls_key = open(tls_key_path, "r").read()

    body = "API_KEY: {}\nTLS_KEY_PATH: {}\nTLS_KEY: {}" \
        .format(api_key, tls_key_path, tls_key)

    r = Response(response=body, status=200, mimetype="text/plain")
    return r

if __name__ == "__main__":
    app.run(debug=False, host='0.0.0.0', port=int(os.environ.get('PORT', 8080)))
