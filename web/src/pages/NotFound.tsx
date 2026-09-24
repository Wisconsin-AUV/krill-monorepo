import { Button } from '@/components/ui/Button'
import { Heading } from '@/components/ui/Heading'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text } from '@/components/ui/Text'

export function NotFound() {
  return (
    <PageContentBlock title="Not found · Krill">
      <div className="py-24 text-center">
        <p className="text-sm font-semibold text-sky-600 dark:text-sky-400">404</p>
        <Heading className="mt-2">Page not found</Heading>
        <Text className="mt-2">That page does not exist or was deleted.</Text>
        <Button to="/" className="mt-8">
          Back home
        </Button>
      </div>
    </PageContentBlock>
  )
}
