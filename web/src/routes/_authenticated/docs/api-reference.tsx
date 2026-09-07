import { createFileRoute } from '@tanstack/react-router'
import { RedocStandalone } from 'redoc'

export const Route = createFileRoute('/_authenticated/docs/api-reference')({
  component: ApiReference,
})

function ApiReference() {
  return (
    <div className="h-[calc(100vh-8rem)]">
      <RedocStandalone
        specUrl="/openapi/api.json"
        options={{
          nativeScrollbars: true,
          hideDownloadButton: true,
          theme: {
            colors: {
              primary: { main: '#3b82f6' },
            },
          },
        }}
      />
    </div>
  )
}