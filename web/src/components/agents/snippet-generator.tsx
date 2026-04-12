/**
 * AgentSnippetGenerator — generates connection code snippets for an A2A agent.
 *
 * Language tabs: curl / Python / TypeScript / Go.
 * Templates are parameterized with the agent's endpoint URL and auth scheme.
 */

import { useState } from 'react'
import { CopyButton } from '@/components/ui/copy-button'

interface AgentSnippetGeneratorProps {
  endpointUrl: string
  authSchemes?: string[]
}

type Language = 'curl' | 'python' | 'typescript' | 'go'

const LANGUAGES: { value: Language; label: string }[] = [
  { value: 'curl', label: 'cURL' },
  { value: 'python', label: 'Python' },
  { value: 'typescript', label: 'TypeScript' },
  { value: 'go', label: 'Go' },
]

function authHeader(scheme: string | undefined): string {
  switch (scheme?.toLowerCase()) {
    case 'bearer':
    case 'oauth2':
    case 'openidconnect':
      return 'Bearer YOUR_TOKEN'
    case 'apikey':
      return 'ApiKey YOUR_API_KEY'
    default:
      return 'Bearer YOUR_TOKEN'
  }
}

function generateSnippet(lang: Language, url: string, auth: string): string {
  const taskPayload = {
    jsonrpc: '2.0',
    id: '1',
    method: 'tasks/send',
    params: {
      id: 'task-001',
      message: {
        role: 'user',
        parts: [{ kind: 'text', text: 'Hello, agent!' }],
      },
    },
  }

  const jsonStr = JSON.stringify(taskPayload, null, 2)

  switch (lang) {
    case 'curl':
      return `curl -X POST "${url}" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: ${auth}" \\
  -d '${JSON.stringify(taskPayload)}'`

    case 'python':
      return `import httpx

response = httpx.post(
    "${url}",
    headers={
        "Content-Type": "application/json",
        "Authorization": "${auth}",
    },
    json=${jsonStr.replace(/"/g, '"')},
)
print(response.json())`

    case 'typescript':
      return `const response = await fetch("${url}", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "Authorization": "${auth}",
  },
  body: JSON.stringify(${jsonStr}),
});
const result = await response.json();
console.log(result);`

    case 'go':
      return `package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"io"
)

func main() {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "tasks/send",
		"params": map[string]interface{}{
			"id": "task-001",
			"message": map[string]interface{}{
				"role": "user",
				"parts": []map[string]interface{}{
					{"kind": "text", "text": "Hello, agent!"},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "${url}", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "${auth}")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	result, _ := io.ReadAll(resp.Body)
	fmt.Println(string(result))
}`
  }
}

export function AgentSnippetGenerator({ endpointUrl, authSchemes }: AgentSnippetGeneratorProps) {
  const [lang, setLang] = useState<Language>('curl')
  const primaryScheme = authSchemes?.[0]
  const auth = authHeader(primaryScheme)
  const snippet = generateSnippet(lang, endpointUrl, auth)

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold">Connection Snippet</h3>
      <p className="text-xs text-muted-foreground">
        Send a task to this agent using the A2A protocol.
      </p>

      {/* Language tabs */}
      <div className="flex items-center gap-1 rounded-lg border p-1 w-fit">
        {LANGUAGES.map((l) => (
          <button
            key={l.value}
            type="button"
            onClick={() => setLang(l.value)}
            className={`rounded-md px-3 py-1 text-xs font-medium transition-colors ${
              lang === l.value
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:text-foreground hover:bg-accent'
            }`}
          >
            {l.label}
          </button>
        ))}
      </div>

      {/* Generated snippet */}
      <div className="relative rounded-md bg-muted overflow-hidden">
        <div className="absolute top-2 right-2 z-10">
          <CopyButton value={snippet} label="Copy snippet" />
        </div>
        <pre className="p-3 pr-12 text-xs font-mono overflow-x-auto whitespace-pre">
          {snippet}
        </pre>
      </div>
    </div>
  )
}
