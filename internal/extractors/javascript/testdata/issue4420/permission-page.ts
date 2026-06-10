export const PermissionPage = {
  Buildings: 'buildings',
  Units: 'units',
  Equipment: 'equipment',
  EquipmentTypes: 'equipment_types',
  Devices: 'devices',
  Users: 'users',
  Roles: 'roles',
  Groups: 'groups',
  Jurisdictions: 'jurisdictions',
  CoreAdmin: 'core-admin',
  Contacts: 'contacts',
  Checklists: 'checklists',
  Notes: 'notes',
  WitnessingCompanies: 'witnessing-companies',
  ContractingCompanies: 'contracting-companies',
  InspectionCompanies: 'inspection-companies',
  EmailTemplates: 'email-templates',
  ContractProposals: 'contract-proposal',
  Routes: 'routes',
  RescheduleRequests: 'reschedule_requests',
  Clients: 'clients',
  Permits: 'permits',
  DocumentTemplates: 'document-templates',
  Inspections: 'inspection-results',
  Schedule: 'scheduling',
  Deficiencies: 'deficiencies',
  UserProfile: 'profile-config',
  InspectionsOverviewReport: 'inspections-overview',
  CorrespondenceHistoryReport: 'correspondence-history',
  BuildingDeviceStatusReport: 'building-device-status-report',
  Reports: 'reports',
  MaintenanceEvaluationContent: 'me-content',
  Aocs: 'aocs',
  AocHarvest: 'aoc-harvest',
  Elv3: 'elv3',
  DobSync: 'sync',
} as const;

export type PermissionPage = (typeof PermissionPage)[keyof typeof PermissionPage];

export const SAFE_METHODS: ReadonlySet<string> = new Set(['GET', 'HEAD', 'OPTIONS']);

export function isSafeMethod(method: string): boolean {
  return SAFE_METHODS.has(method.toUpperCase());
}
