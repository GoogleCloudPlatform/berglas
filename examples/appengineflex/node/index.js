const fs = require('fs');
const express = require('express');
const app = express();

app.get('/', (req, res) => {
  const apiKey = process.env.API_KEY;
  const tlsKeyPath = process.env.TLS_KEY;

  let tlsKey = 'file does not exist';
  if (tlsKeyPath && fs.existsSync(tlsKeyPath)) {
    tlsKey = fs.readFileSync(tlsKeyPath, 'utf8');
  }

  let resp = `API_KEY: ${apiKey}\n`;
  resp += `TLS_KEY_PATH: ${tlsKeyPath}\n`;
  resp += `TLS_KEY: ${tlsKey}\n`;

  res.type('text/plain');
  res.send(resp);
});

const port = process.env.PORT || 8080;
app.listen(port);
