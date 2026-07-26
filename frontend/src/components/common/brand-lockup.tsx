import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { cn } from '@/lib/utils'

export function BrandLockup({ className, title }: { className?: string; title?: string }) {
  const { t } = useTranslation()
  const accessibleTitle = title ?? t('login.title')

  return (
    <Link to="/" className={cn('flex justify-center', className)}>
      <img
        src="/logo-brand.png"
        alt="ADTEC"
        className="h-28 w-auto max-w-full object-contain"
      />
      <h1 className="sr-only">{accessibleTitle}</h1>
    </Link>
  )
}
