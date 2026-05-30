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

output storageId string = storageAccount.id
output blobEndpoint string = storageAccount.properties.primaryEndpoints.blob
