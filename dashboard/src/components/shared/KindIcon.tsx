import {
  Box, Code, Component, Database, File, FileText, Folder,
  FunctionSquare, Globe, Hash, Layers, LayoutGrid, Link2,
  MessageSquare, Network, Package, Play, Puzzle, Radio,
  Server, Settings, Shapes, Table, Workflow, Zap,
} from 'lucide-react'
import type { EntityKind } from '@/types/api'

interface KindIconProps {
  kind: EntityKind
  className?: string
}

const KIND_ICONS: Partial<Record<EntityKind, React.FC<{ className?: string }>>> = {
  Function:     FunctionSquare,
  Class:        Box,
  Component:    Component,
  Schema:       Shapes,
  Route:        Network,
  Endpoint:     Globe,
  Service:      Server,
  DataAccess:   Database,
  Datastore:    Database,
  Model:        Table,
  Queue:        MessageSquare,
  MessageTopic: Radio,
  ExternalAPI:  Link2,
  Document:     FileText,
  Heading:      Hash,
  Variable:     Hash,
  Reference:    Link2,
  Pattern:      Puzzle,
  View:         LayoutGrid,
  UIComponent:  LayoutGrid,
  JSX:          Code,
  Stylesheet:   Layers,
  Event:        Zap,
  InfraResource: Settings,
  CodeBlock:    Code,
  Config:       Settings,
  Process:      Workflow,
  AgentPattern: Play,
  Project:      Folder,
  Operation:    Play,
  Evolution:    Package,
  External:     Globe,
  ScopeUnknown: File,
}

export function KindIcon({ kind, className = 'w-4 h-4' }: KindIconProps) {
  const Icon = KIND_ICONS[kind] ?? File
  return <Icon className={className} aria-label={kind} />
}
