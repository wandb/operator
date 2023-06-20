const SmallDot: React.FC = () => (
  <div className="my-[3px] h-[6px] w-[6px] shrink rounded-full bg-yellow-400 transition-transform" />
)

const BigDot: React.FC = () => (
  <div className="my-[2px] h-[10px] w-[10px] shrink rounded-full bg-yellow-400 transition-transform" />
)

const Column: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className={'mx-[5px] flex flex-col items-center'}>{children}</div>
)

const Container: React.FC<{
  children: React.ReactNode
}> = ({ children }) => (
  <div className="flex scale-[.5] cursor-pointer justify-center overflow-hidden">
    {children}
  </div>
)

export const WBLogo: React.FC = () => (
  <Container>
    <Column>
      <SmallDot />
      <BigDot />
      <SmallDot />
      <BigDot />
    </Column>
    <div className="mt-[5px]">
      <Column>
        <SmallDot />
        <SmallDot />
        <BigDot />
        <SmallDot />
      </Column>
    </div>
    <Column>
      <SmallDot />
      <BigDot />
      <SmallDot />
      <SmallDot />
    </Column>
  </Container>
)
