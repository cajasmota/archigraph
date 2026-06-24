@description('Storage account name')
param storageName string
param location string = resourceGroup().location

var tags = {
  env: 'prod'
}

resource storageAccount 'Microsoft.Storage/storageAccounts@2022-09-01' = {
  name: storageName
  location: location
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  tags: tags
}

resource blobService 'Microsoft.Storage/storageAccounts/blobServices@2022-09-01' = {
  name: '${storageAccount.name}/default'
  properties: {
    deleteRetentionPolicy: {
      enabled: true
    }
  }
  dependsOn: [
    storageAccount
  ]
}

module network './modules/network.bicep' = {
  name: 'networkDeploy'
  params: {
    storageId: storageAccount.id
    location: location
  }
}

resource existingVault 'Microsoft.KeyVault/vaults@2022-07-01' existing = {
  name: 'shared-kv'
}

// Registry module reference (full ACR OCI ref).
module avmStorage 'br:myreg.azurecr.io/bicep/modules/storage:1.2.0' = {
  name: 'avmStorageDeploy'
  params: {
    location: location
  }
}

// Public registry module via the bicepconfig `public` alias.
module avmVault 'br/public:avm/res/key-vault/vault:0.6.1' = {
  name: 'avmVaultDeploy'
}

// Template-spec reference.
module sharedTs 'ts:00000000-0000-0000-0000-000000000000/shared-rg/networkSpec:2.0' = {
  name: 'sharedTsDeploy'
}

output storageId string = storageAccount.id
output blobEndpoint string = storageAccount.properties.primaryEndpoints.blob
