# DeadDrop frontend

The public DeadDrop website is a static Next.js App Router application. It has no runtime server, database connection, authentication, or environment variables.

## Development

```bash
npm ci
npm run dev
```

Open `http://localhost:3000`.

## Production export

```bash
npm run lint
npm run build
```

Next.js writes the deployable site to `out/`.

## Render Static Site

- Root Directory: `frontend`
- Build Command: `npm ci && npm run build`
- Publish Directory: `out`
- Environment Variables: none

The FastAPI dashboard is a separate private web service. Never place `OWNER_TOKEN`, `WORKER_TOKEN`, or `DATABASE_URL` in this frontend.
