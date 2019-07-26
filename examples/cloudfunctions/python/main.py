import os

import berglas.auto  # noqa


def handler(request):
    api_key = os.environ.get('API_KEY')
    tls_key_path = os.environ.get('TLS_KEY')

    tls_key = "file does not exist"
    if tls_key_path and os.path.isfile(tls_key_path):
        tls_key = open(tls_key_path, "r").read()

    body = "API_KEY: {}\nTLS_KEY_PATH: {}\nTLS_KEY: {}" \
        .format(api_key, tls_key_path, tls_key)

    return body
