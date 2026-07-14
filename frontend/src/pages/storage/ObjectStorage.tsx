import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import PageHeader from '@/components/common/PageHeader'
import BucketList from './BucketList'
import ObjectList from './ObjectList'
import type { Bucket } from '@/types/api'

export default function ObjectStorage() {
  const { t } = useTranslation()
  const [selectedBucket, setSelectedBucket] = useState<Bucket | null>(null)
  const [tenantId, setTenantId] = useState<number | undefined>()

  return (
    <div>
      <PageHeader title={t('nav.storage')} />
      {selectedBucket && tenantId ? (
        <ObjectList bucket={selectedBucket} tenantId={tenantId} onBack={() => setSelectedBucket(null)} />
      ) : (
        <BucketList onSelect={(b) => { setSelectedBucket(b); setTenantId(undefined) }} />
      )}
    </div>
  )
}
