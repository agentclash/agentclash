const express = require('express');
const { Pool } = require('pg');
const config = require('./config.json');

const app = express();
app.use(express.json());

const pool = new Pool({
  connectionString: process.env.DATABASE_URL || `postgres://${config.database.user}@${config.database.host}:${config.database.port}/${config.database.name}`,
  ssl: config.database.ssl ? { rejectUnauthorized: false } : false,
});

app.get('/health', async (req, res) => {
  try {
    await pool.query('SELECT 1');
    res.json({ status: 'ok' });
  } catch (err) {
    res.status(500).json({ status: 'error', message: err.message });
  }
});

app.get('/api/items', async (req, res) => {
  try {
    const result = await pool.query('SELECT * FROM items ORDER BY created_at DESC');
    res.json(result.rows);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

const port = process.env.PORT || config.server.port;
app.listen(port, config.server.host, () => {
  console.log(`Server running on ${config.server.host}:${port}`);
});
