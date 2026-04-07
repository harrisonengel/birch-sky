export type NodeType =
  | 'service'
  | 'datastore'
  | 'queue'
  | 'external'
  | 'client';

export type EdgeType =
  | 'sync'
  | 'async'
  | 'reads'
  | 'writes'
  | 'readwrite';

export interface ArchNode {
  id: string;
  type: NodeType;
  label: string;
  description?: string;
  tags?: string[];
  meta?: {
    language?: string;
    repo_path?: string;
    owner?: string;
    confidence?: 'high' | 'low';
    [key: string]: unknown;
  };
}

export interface ArchEdge {
  from: string;
  to: string;
  type: EdgeType;
  label?: string;
  meta?: {
    protocol?: string;
    confidence?: 'high' | 'low';
    [key: string]: unknown;
  };
}

export interface ArchGraph {
  version: string;
  updated_at: string;
  updated_by: string;
  nodes: ArchNode[];
  edges: ArchEdge[];
}
