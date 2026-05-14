# Detailed Deployment Guide

This guide covers how to deploy your own private DeadDrop instance for free using Render and Supabase.

## Prerequisites
- A GitHub account.
- A free account on Supabase.
- A free account on Render.

---

## Step 1: Database Setup (Supabase)
DeadDrop requires a persistent PostgreSQL database. Supabase provides a powerful free tier.

1.  **Create a New Project**:
    - Log into Supabase and click **New Project**.
    - Choose a name (e.g., `deaddrop-db`) and a secure password.
    - Select a region close to you.
2.  **Configure Connection Pooling**:
    - Once the project is ready, go to **Settings (cog icon) > Database**.
    - Scroll down to the **Connection Pooler** section.
    - **IMPORTANT**: Set the **Pool Mode** to `Transaction`.
    - Find the **Connection String** box, ensure `Node.js` or `URI` is selected, and copy the string.
    - **CRITICAL**: Use the **Pooled** connection string (typically ends with port `6543`). This ensures compatibility with Render's networking and handles many short-lived connections efficiently.
    - *Example format:* `postgresql://postgres.[ID]:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:6543/postgres`

---

## Step 2: Server Setup (Render)
Render will host the DeadDrop FastAPI server in a secure Docker container.

1.  **Fork the Repository**:
    - Fork DeadDrop to your own GitHub account.
2.  **Create a Blueprint**:
    - Log into Render and click **New > Blueprint**.
    - Connect your GitHub account and select your DeadDrop fork.
3.  **Configure Environment Variables**:
    Render will read the `render.yaml` file and ask you to fill in the following:
    - `DATABASE_URL`: Paste your Supabase Pooled Connection String from Step 1.
    - `OWNER_TOKEN`: Generate a long, random secret string (e.g., `openssl rand -base64 32`). This generates a 32-byte cryptographically secure secret that is encoded in Base64 for easy pasting. This is your password for the browser dashboard.
    - `WORKER_TOKEN`: Generate another unique secret string. This is what your local PC's worker will use to talk to the server.
    - `SECURE_COOKIES`: Set this to `true`. This enforces that login cookies are only sent over encrypted HTTPS connections.
4.  **Deploy**:
    - Click **Apply**. Render will build the Docker image and deploy the service.
    - Once finished, you will get a URL like `https://deaddrop-xxxx.onrender.com`.

---

## Step 3: Local Worker Setup
The worker runs on your local machine and polls the server for tasks.

1.  **Clone your fork** locally.
2.  **Configure the Workspace**:
    - Pick the one directory the worker should run inside. It may be a git repo, a subdirectory of a repo, or a plain folder.
    - Use `repo_alias` / `--repo-alias` value `default`; browser-created jobs always route to `local` / `default`.
3.  **Start the Worker**:
    ```bash
    cd worker
    go run . run \
      --server https://your-app-name.onrender.com \
      --token YOUR_WORKER_TOKEN \
      --worker local \
      --repo /absolute/path/to/your/workspace \
      --repo-alias default \
      --agent gemini
    ```
    The worker will register the fixed workspace with the server and start polling.

---

## Security Aspect: How DeadDrop Protects You

DeadDrop is designed with a "Trust but Verify" model. Unlike other agents that might have broad access to your machine, DeadDrop is restricted by design:

### 1. Zero Inbound Connections
The server never connects to your PC. Your PC's worker makes outbound requests to the server to check for jobs. This means you don't need to open ports on your router or use tunnels like Ngrok.

### 2. Bearer Token Authentication
Every single request is guarded by high-entropy tokens:
- **Owner Token**: Protects the dashboard. Only you can see your tasks.
- **Worker Token**: Protects the worker APIs. Only your local machine can claim and update jobs.
- All comparisons are done using constant-time logic (`compare_digest`) to prevent hackers from guessing tokens via timing side-channels.

### 3. CSRF & Secure Cookies
- **Double Submit Cookie**: Every form in the dashboard is protected by a CSRF token. This prevents malicious websites from trying to "drop a task" into your inbox while you're logged in elsewhere.
- **Strict HTTPS**: In production, cookies are flagged as `Secure` and `HttpOnly`, meaning they are never sent over plain text and cannot be stolen by malicious browser scripts.

### 4. Worker Hardening
- **No Root Execution**: The Go worker will refuse to start if run as `root`. This limits the potential damage if a malicious task prompt were somehow executed.
- **Path Isolation**: The worker only operates inside the directories you explicitly whitelist with `--repo` or your manifest. Git status and diff capture are scoped to the configured workspace path.
- **No Auto-Commit**: By default, the system captures a `git diff` but never runs `git commit`. You are the final gatekeeper—you review the diff on the dashboard and apply it manually on your machine.

---

## Cost Analysis
- **Supabase**: Free Tier (500MB DB) is more than enough for thousands of DeadDrop tasks.
- **Render**: Free Tier (Web Service) is sufficient for personal use. Note that the Free Tier "sleeps" after 15 minutes of inactivity; the first request after a sleep may take 30 seconds to load.
## Token Management Best Practices

To keep your DeadDrop instance secure, follow these guidelines for managing your `OWNER_TOKEN` and `WORKER_TOKEN`:

### 1. Avoid Persistent .env Files
While `.env` files are convenient for local development, they are a security risk if left on disk. 
- **Production**: Never use `.env` files. Use Render's built-in **Environment Variables** interface. These are stored securely in memory and never touch the server's disk.
- **Local**: Instead of creating a `.env` file, set the tokens in your current terminal session:
  ```bash
  export OWNER_TOKEN="your_secret_here"
  export WORKER_TOKEN="your_secret_here"
  ```
  This ensures that when you close your terminal, the secrets are gone from memory.

### 2. Protect the Worker from the Agent
If you configure a worker to run an agent (like Gemini) inside a repository, the agent has the same file permissions as the worker. 
- **CRITICAL**: Never add the `DeadDrop` repository itself to your worker's manifest. If you do, a malicious prompt could trick the agent into reading your `DeadDrop` configuration or `.env` files.
- **Isolation**: Only add repositories that contain the code you want to work on. Keep your "Mission Control" (the DeadDrop server and worker code) separate from the "Workspaces" (your project repos).

### 3. Rotate Tokens Regularly
If you suspect a token has been leaked, rotate it immediately:
1. Update the variable on Render.
2. Restart your local worker with the new `WORKER_TOKEN`.
3. Log back into the dashboard with the new `OWNER_TOKEN`.
