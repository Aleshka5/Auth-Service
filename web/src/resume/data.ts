export type TreeItem = {
  id: string
  title: string
  summary?: string
  meta?: string
  chips?: string[]
  href?: string
  children?: TreeItem[]
}

export type ArchitectureSolution = {
  slug: string
  title: string
  summary: string
  href?: string
}

export const profile = {
  name: 'Alexey Filenkov',
  title: 'Senior AI Agents',
  years: '5 years',
  english: 'B1 (Intermediate)',
  lede:
    'I build AI agents for classification, extraction, chat, and reporting — Plan-and-Execute, ReAct, RAG, and vector search. Open a node to go from the business result to the implementation.',
  contacts: [
    { label: 'Email', value: 'fad80223@gmail.com', href: 'mailto:fad80223@gmail.com' },
    { label: 'Telegram', value: '@alekseyfilenkov', href: 'https://t.me/alekseyfilenkov' },
    { label: 'GitHub', value: 'github.com/Aleshka5', href: 'https://github.com/Aleshka5' },
  ],
} as const

export const stackTree: TreeItem[] = [
  {
    id: 'stack-agents',
    title: 'AI agents',
    summary: 'Agents that route work, extract facts, and write reports without a human in every loop.',
    chips: ['LangChain', 'LangGraph', 'RAG', 'Faiss'],
    children: [
      {
        id: 'stack-graphs',
        title: 'Plan-and-Execute & ReAct',
        summary: 'Graph-shaped runs: plan, tool calls, memory between reports.',
        children: [
          {
            id: 'stack-mcp',
            title: 'MCP tools',
            summary: 'Tools live in a separate Model Context Protocol service so graphs stay thin and reusable.',
            chips: ['MCP', 'FastAPI'],
          },
        ],
      },
      {
        id: 'stack-rag',
        title: 'RAG & embeddings',
        summary: 'Domain retrieval so the model answers from the corpus, not from hope.',
        chips: ['Faiss', 'HuggingFace', 'PyTorch'],
      },
      {
        id: 'stack-prompt',
        title: 'Prompt-first extraction',
        summary: 'Multi-label mail routing and NER shipped on prompt engineering before any fine-tune.',
        chips: ['Mistral Large', 'NER', 'PyMorphy'],
      },
    ],
  },
  {
    id: 'stack-backend',
    title: 'Backend',
    summary: 'APIs other services can trust: sessions, roles, and queues.',
    chips: ['Python', 'FastAPI', 'Go'],
    children: [
      {
        id: 'stack-data',
        title: 'Data & buses',
        summary: 'Postgres for truth, Redis for sessions and limits, Kafka or RabbitMQ when work is async.',
        chips: ['PostgreSQL', 'Redis', 'Kafka', 'RabbitMQ'],
      },
      {
        id: 'stack-auth-impl',
        title: 'Auth contracts',
        summary: 'Google OAuth2, httpOnly cookie on .filenkov.store, gRPC Validate for every product host.',
        chips: ['OAuth2', 'gRPC', 'JWT session'],
      },
    ],
  },
  {
    id: 'stack-infra',
    title: 'Infra',
    summary: 'One domain, TLS at the edge, object storage shared by the house.',
    chips: ['Caddy', 'Docker', 'MinIO'],
    children: [
      {
        id: 'stack-edge',
        title: 'Edge & identity',
        summary: 'Caddy terminates HTTPS with Let’s Encrypt. DNS points every subdomain at the same gate.',
        chips: ['Let’s Encrypt', 'DNS'],
      },
      {
        id: 'stack-roles',
        title: 'Per-service roles',
        summary: 'ADMIN / FAMILY / STRANGER / BLOCKED as enums — block storage without locking resume.',
        chips: ['Role Manager'],
      },
    ],
  },
]

export const projectTree = (showHermes: boolean): TreeItem[] => {
  const items: TreeItem[] = [
    {
      id: 'proj-auth',
      title: 'Auth hub',
      meta: 'filenkov.store',
      summary: 'One Google sign-in. Other Filenkov sites ask this hub who you are — no tokens in the URL.',
      children: [
        {
          id: 'proj-auth-cookie',
          title: 'Session on the cookie',
          summary: 'httpOnly auth_session on .filenkov.store. Products call gRPC Validate with the session id.',
          chips: ['Go', 'gRPC', 'Redis'],
          children: [
            {
              id: 'proj-auth-roles',
              title: 'Whitelist & roles',
              summary:
                'Each host gets a field projection. New accounts start as STRANGER. BLOCKED is per service, not global.',
              chips: ['Postgres enums', 'Rate limit by IP'],
            },
          ],
        },
      ],
    },
    {
      id: 'proj-storage',
      title: 'Storage',
      meta: 'storage.filenkov.store',
      summary: 'Home cloud for family files — same sign-in, roles that stay on this host.',
      href: 'https://storage.filenkov.store',
      children: [
        {
          id: 'proj-storage-roles',
          title: 'Family-aware access',
          summary: 'ADMIN, FAMILY, STRANGER, or BLOCKED on storage_roles only.',
          chips: ['FastAPI', 'Postgres', 'MinIO'],
        },
      ],
    },
    {
      id: 'proj-llm',
      title: 'LLM',
      meta: 'llm.filenkov.store',
      summary: 'Personal LLM workspace behind the same auth gate.',
      href: 'https://llm.filenkov.store',
      children: [
        {
          id: 'proj-llm-gate',
          title: 'Auth at the edge',
          summary: 'Caddy + /auth/validate. The model stack stays off the public role list until it needs one.',
        },
      ],
    },
    {
      id: 'proj-resume',
      title: 'Resume studio',
      meta: 'vacancies',
      summary: 'AI resume and cover-letter generator: chat a vacancy, get a tailored PDF.',
      children: [
        {
          id: 'proj-resume-flow',
          title: 'Spec-driven pipeline',
          summary: 'Knowledge base → agent graph → LaTeX artifact. Humans stay in the loop on the weak cases.',
          chips: ['LangGraph', 'FastAPI', 'XeLaTeX'],
          children: [
            {
              id: 'proj-resume-store',
              title: 'Files & identity',
              summary: 'Postgres for chats, MinIO for knowledge and PDFs, Google through this hub.',
              chips: ['Postgres', 'MinIO', 'gRPC Validate'],
            },
          ],
        },
      ],
    },
  ]

  if (showHermes) {
    items.push({
      id: 'proj-hermes',
      title: 'Hermes',
      meta: 'filenkov.store/hermes/',
      summary: 'Internal tools. Off the public list.',
      href: '/hermes/',
    })
  }

  return items
}

export const experienceTree: TreeItem[] = [
  {
    id: 'exp-sber',
    title: 'Sber',
    meta: '2023 — 2026 · 3 years',
    summary: 'Middle+ AI Agents. Automated reporting: ~1M RUB/year saved at 1,000 reports a week.',
    chips: ['LangGraph', 'FastAPI', 'PostgreSQL', 'Kafka', 'Faiss', 'RAG'],
    children: [
      {
        id: 'exp-sber-cost',
        title: '2× cheaper tokens',
        summary: 'Graph rewrite cut a report from 60,000 to 30,000 tokens.',
        children: [
          {
            id: 'exp-sber-graph',
            title: 'Fewer loops, same quality',
            summary: 'Tighter Plan-and-Execute graph, tools moved out so the model stopped re-discovering them.',
          },
        ],
      },
      {
        id: 'exp-sber-memory',
        title: 'Reports that remember',
        summary: 'Memory links reports to each other. RAG covers the domain so later runs stay consistent.',
        chips: ['Memory', 'Faiss', 'RAG'],
      },
      {
        id: 'exp-sber-mcp',
        title: 'Tools as MCP',
        summary: 'Agent tools live in a separate MCP service — two product services, one tool layer.',
        chips: ['MCP'],
      },
    ],
  },
  {
    id: 'exp-aero',
    title: 'AeroClub',
    meta: '2021 — 2023 · 2 years',
    summary: 'Middle LLM Engineer. Mail routing fully automated: 85% precision, 80% recall on new data.',
    chips: ['LangChain', 'FastAPI', 'LabelStudio', 'RabbitMQ', 'scikit-learn'],
    children: [
      {
        id: 'exp-aero-mlc',
        title: 'Multi-label mail routing',
        summary: 'Mistral Large, prompt-only. No fine-tune for the production scores.',
        children: [
          {
            id: 'exp-aero-cm',
            title: 'Task-shaped confusion matrix',
            summary: 'Floating false-positive classes became visible, so prompt iterations hit the weak labels first.',
            chips: ['scikit-learn'],
          },
        ],
      },
      {
        id: 'exp-aero-ner',
        title: 'NER at 80%+',
        summary: 'Entity extraction on new mail, again prompt-only, then shipped behind FastAPI.',
      },
      {
        id: 'exp-aero-hitl',
        title: 'Human-in-the-loop framework',
        summary: 'One framework for agent logic, LabelStudio for hard cases, RabbitMQ for the async loop.',
        chips: ['LabelStudio', 'RabbitMQ'],
      },
    ],
  },
]

export const education = [
  {
    school: 'MAI (Moscow Aviation Institute)',
    detail: 'Data Science and Machine Learning. Dissertation: Diffusion Transformer with short-term memory.',
    when: '2024 — 2026',
  },
  {
    school: "MPEI (National Research University 'MPEI')",
    detail: 'Bachelor’s, Computer Systems and Engineering. Thesis: Telegram bot with a RAG system.',
    when: '2020 — 2024',
  },
] as const

export const architectureSolutions: ArchitectureSolution[] = [
  {
    slug: 'auth-gateway',
    title: 'Auth Gateway',
    summary: 'Google OAuth2 hub at filenkov.store/auth. Issues the cookie every other service trusts.',
    href: '/auth',
  },
  {
    slug: 'caddy-https',
    title: 'Caddy HTTPS',
    summary: 'TLS at the edge with Let’s Encrypt. One reverse proxy for auth, SPA, and product hosts.',
  },
  {
    slug: 'auth-db',
    title: 'Auth DB',
    summary: 'Postgres users and per-service role enums. Source of truth for Validate projections.',
  },
  {
    slug: 'auth-cache',
    title: 'Auth Cache',
    summary: 'Redis sessions, OAuth state, IP rate limits, and the redirect whitelist.',
  },
  {
    slug: 'role-manager',
    title: 'Role Manager',
    summary: 'Admin UI to raise or BLOCK a role on one host without touching the others.',
  },
  {
    slug: 'storage',
    title: 'Storage',
    summary: 'Home file cloud. Validates through gRPC, stores blobs on the shared MinIO.',
    href: 'https://storage.filenkov.store',
  },
  {
    slug: 'llm',
    title: 'LLM',
    summary: 'Personal model workspace. Same cookie, own database, auth at /auth/validate.',
    href: 'https://llm.filenkov.store',
  },
  {
    slug: 'resume',
    title: 'Resume Page',
    summary: 'Vacancy-aware resume and cover-letter studio. Will grow its own architecture page.',
  },
  {
    slug: 'minio',
    title: 'Common MinIO S3',
    summary: 'Shared object store: home_storage, resume artifacts, and service backups.',
  },
  {
    slug: 'dns',
    title: 'DNS',
    summary: 'filenkov.store and the product subdomains. The map starts here.',
  },
]

export function solutionBySlug(slug: string | undefined): ArchitectureSolution | undefined {
  if (!slug) return undefined
  return architectureSolutions.find((item) => item.slug === slug)
}
