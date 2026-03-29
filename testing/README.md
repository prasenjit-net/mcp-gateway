# Testing — Sample Specs

Test OpenAPI 3 specs for trying out the MCP Gateway locally.

---

## task-manager.yaml

A fully-documented **Task Manager API** with projects, tasks, comments, and
search. Every operation has rich descriptions so MCP tools generated from it
are immediately useful to AI agents.

### Operations (MCP tool names after upload)

| Tool name | Method | Path | What it does |
|-----------|--------|------|-------------|
| `listProjects` | GET | `/projects` | List all projects (paginated, sortable) |
| `createProject` | POST | `/projects` | Create a new project |
| `getProject` | GET | `/projects/{projectId}` | Get project details + task counts by status |
| `updateProject` | PATCH | `/projects/{projectId}` | Rename, redescribe, recolour a project |
| `deleteProject` | DELETE | `/projects/{projectId}` | Delete project and all its tasks |
| `listTasks` | GET | `/projects/{projectId}/tasks` | List tasks — filter by status/priority/assignee/due date |
| `createTask` | POST | `/projects/{projectId}/tasks` | Create a task with title, priority, due date, assignee |
| `getTask` | GET | `/projects/{projectId}/tasks/{taskId}` | Get full task details |
| `updateTask` | PATCH | `/projects/{projectId}/tasks/{taskId}` | Update status, priority, assignee, description |
| `deleteTask` | DELETE | `/projects/{projectId}/tasks/{taskId}` | Delete a task |
| `listComments` | GET | `/projects/{projectId}/tasks/{taskId}/comments` | List comments on a task |
| `addComment` | POST | `/projects/{projectId}/tasks/{taskId}/comments` | Add a comment to a task |
| `searchTasks` | GET | `/search` | Full-text search across all projects |
| `getWorkspaceStats` | GET | `/stats` | Aggregate stats: tasks by status/priority, overdue count |

### Upload to the gateway

1. Start the gateway: `make run` (or `make dev`)
2. Open `http://localhost:8080/_ui/specs`
3. Click **Upload New Spec**
4. Fill in:
   - **Name**: `Task Manager`
   - **Upstream Base URL**: `http://localhost:8081`
   - **Spec file**: `testing/task-manager.yaml`
   - **Auth**: none (the mock server requires no auth)
5. Click Upload — 14 tools will be registered

### Try a mock upstream

Use [httpbin](https://httpbin.org) or [WireMock](https://wiremock.org) as a
stand-in backend. The `docker-compose.yml` in the project root starts an
httpbin instance at `http://localhost:8081`.

```bash
docker compose up mock-api
```

> httpbin doesn't implement the task manager API — it echoes requests back.
> That's enough to verify that the gateway is correctly building and
> forwarding HTTP requests with the right paths, params, and bodies.

### Chat test prompts

Once the spec is uploaded and the mock is running, open the Chat page
(`/_ui/chat`) and try these prompts:

```
"Create a project called 'Q3 Launch' with colour #f59e0b"

"List all the projects"

"Create a high-priority task called 'Fix login bug' in project proj_01,
 assign it to alice, due 2025-08-15"

"Show me all in-progress tasks for project proj_01"

"Search for tasks about 'login'"

"Mark task task_42 in project proj_01 as done"

"Add a comment to task task_42 in project proj_01: 
 'Fixed in PR #12, deployed to staging'"

"Give me a summary of the workspace stats"
```
