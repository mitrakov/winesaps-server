package ru.mitrakov.self.rush.tester

import akka.actor._

import scala.concurrent.ExecutionContext.Implicits._
import scala.concurrent.duration._
import scala.language.postfixOps

/**
 * Created by mitrakov on 05.07.2017
 */
class Dispatcher(crcIds: List[Int], router: ActorRef) extends Actor {
  private var i = 0

  override def receive: Receive = {
    case _ => context.system.scheduler.schedule(3 seconds, (200 / crcIds.length) milliseconds, new Runnable {
      override def run(): Unit = {
        router ! Go(crcIds(i))
        i = (i+1) % crcIds.length
      }
    })
  }
}
