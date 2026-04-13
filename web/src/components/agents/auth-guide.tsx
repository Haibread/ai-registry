/**
 * AuthGuide — renders authentication instructions for an agent
 * based on its declared auth scheme(s).
 */

import { Shield } from 'lucide-react'

interface AuthGuideProps {
  schemes: Array<Record<string, string>>
}

function guideForScheme(scheme: Record<string, string>): { title: string; description: string } {
  const name = (scheme.scheme ?? scheme.type ?? '').toLowerCase()

  switch (name) {
    case 'bearer':
      return {
        title: 'Bearer Token',
        description:
          'Include a Bearer token in the Authorization header: `Authorization: Bearer <token>`. Obtain the token from the agent provider.',
      }
    case 'oauth2':
      return {
        title: 'OAuth 2.0',
        description:
          'This agent uses OAuth 2.0. You will need to complete an authorization flow to obtain an access token. Check the agent documentation for the OAuth endpoints and scopes.',
      }
    case 'openidconnect':
      return {
        title: 'OpenID Connect',
        description:
          'This agent uses OpenID Connect for authentication. Discover the OIDC configuration from the provider and complete the authorization code flow with PKCE.',
      }
    case 'apikey':
      return {
        title: 'API Key',
        description:
          'Include your API key in the Authorization header: `Authorization: ApiKey <key>`. Request a key from the agent provider.',
      }
    default:
      return {
        title: name || 'Custom Authentication',
        description:
          'This agent uses a custom authentication scheme. Refer to the agent documentation for details.',
      }
  }
}

export function AuthGuide({ schemes }: AuthGuideProps) {
  if (!schemes || schemes.length === 0) return null

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold flex items-center gap-1.5">
        <Shield className="h-4 w-4" />
        Authentication
      </h3>
      <div className="space-y-2">
        {schemes.map((scheme, i) => {
          const guide = guideForScheme(scheme)
          return (
            <div key={i} className="rounded-md border p-3 text-sm space-y-1">
              <p className="font-medium">{guide.title}</p>
              <p className="text-muted-foreground text-xs">{guide.description}</p>
            </div>
          )
        })}
      </div>
    </div>
  )
}
